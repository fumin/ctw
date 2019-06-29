package main

import (
	"bufio"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

var (
	srcDir = flag.String("s", "", "source directory")
	dstDir = flag.String("d", "", "destination directory")
)

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	if err := run(*srcDir, *dstDir); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(srcDir, dstDir string) error {
	srcs, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return errors.Wrap(err, "")
	}
	for _, srcInfo := range srcs {
		src := srcInfo.Name()
		r, err := os.Open(filepath.Join(srcDir, src))
		if err != nil {
			return errors.Wrap(err, "")
		}
		defer r.Close()
		dstName := strings.TrimSuffix(src, filepath.Ext(src)) + ".atcg"
		w, err := os.Create(filepath.Join(dstDir, dstName))
		if err != nil {
			return errors.Wrap(err, "")
		}
		defer w.Close()
		if err := encode(w, r); err != nil {
			return errors.Wrap(err, "")
		}
	}
	return nil
}

func encode(w io.Writer, r io.Reader) error {
	kill := make(chan struct{})
	defer close(kill)
	src := make(chan byte)
	errc := make(chan error)
	go func() {
		defer close(src)
		err := func() error {
			scanner := bufio.NewScanner(r)
			scanner.Split(bufio.ScanBytes)
			for scanner.Scan() {
				var bt byte = scanner.Bytes()[0]
				var c byte
				switch bt {
				case 'a':
					c = 0
				case 't':
					c = 1
				case 'c':
					c = 2
				case 'g':
					c = 3
				default:
					continue
				}
				select {
				case <-kill:
					return nil
				case src <- c:
				}
			}
			if err := scanner.Err(); err != nil {
				return errors.Wrap(err, "")
			}
			return nil
		}()
		if err != nil {
			select {
			case <-kill:
				return
			case errc <- err:
			}
		}
	}()

	go func() {
		err := func() error {
			buf := []byte{0}
			var bt *byte = &buf[0]
			var shift uint = 0
			for c := range src {
				*bt |= (c << shift)
				// 2 bits for 4 different numbers.
				shift += 2

				if shift == 8 {
					if _, err := w.Write(buf); err != nil {
						return err
					}
					*bt = 0
					shift = 0
				}
			}

			// Write left over bytes.
			if shift > 0 {
				if _, err := w.Write(buf); err != nil {
					return err
				}
			}
			return nil
		}()
		select {
		case <-kill:
			return
		case errc <- err:
		}
	}()

	if err := <-errc; err != nil {
		return errors.Wrap(err, "")
	}
	return nil
}
