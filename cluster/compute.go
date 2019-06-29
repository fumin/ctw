package main

import (
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fumin/ctw"
	"github.com/pkg/errors"
)

var (
	intelligenceType = flag.String("i", "ctw", "intelligence type")
	dataDir          = flag.String("d", "mammals10", "data directory")
)

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	if err := run(*intelligenceType, *dataDir); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(intelligence, dir string) error {
	data, err := listFiles(dir)
	if err != nil {
		return errors.Wrap(err, "")
	}
	distMat, err := distanceMatrix(intelligence, data)
	if err != nil {
		return errors.Wrap(err, "")
	}

	if err := display(data, distMat); err != nil {
		return errors.Wrap(err, "")
	}
	return nil
}

func display(data []string, distMat []float64) error {
	// Print data as a comma separated array.
	buf := bytes.NewBuffer(nil)
	for i, fpath := range data {
		if err := buf.WriteByte('"'); err != nil {
			return errors.Wrap(err, "")
		}

		name := filepath.Base(fpath)
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if _, err := buf.WriteString(base); err != nil {
			return errors.Wrap(err, "")
		}

		if err := buf.WriteByte('"'); err != nil {
			return errors.Wrap(err, "")
		}

		if i == len(data)-1 {
			break
		}
		if err := buf.WriteByte(','); err != nil {
			return errors.Wrap(err, "")
		}
	}
	log.Printf("[%s]", buf.Bytes())

	// Print distance matrix as a comma separated array.
	buf.Reset()
	for i, f := range distMat {
		if _, err := buf.WriteString(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
			return errors.Wrap(err, "")
		}
		if i == len(distMat)-1 {
			break
		}
		if err := buf.WriteByte(','); err != nil {
			return errors.Wrap(err, "")
		}
	}
	log.Printf("[%s]", buf.Bytes())

	return nil
}

func distance(cacher map[string]float64, intelligence, x, y string) (float64, error) {
	xyFname := filepath.Join("/tmp", filepath.Base(x)+filepath.Base(y))
	xy, err := os.Create(xyFname)
	if err != nil {
		return -1, errors.Wrap(err, "")
	}
	defer os.Remove(xy.Name())
	if err := concatFiles(xy, x, y); err != nil {
		return -1, errors.Wrap(err, "")
	}

	kxy, err := complexity(cacher, intelligence, xy.Name())
	if err != nil {
		return -1, errors.Wrap(err, "")
	}
	kx, err := complexity(cacher, intelligence, x)
	if err != nil {
		return -1, errors.Wrap(err, "")
	}
	ky, err := complexity(cacher, intelligence, y)
	if err != nil {
		return -1, errors.Wrap(err, "")
	}

	minxy := kx
	if ky < kx {
		minxy = ky
	}
	maxxy := kx
	if ky > kx {
		maxxy = ky
	}

	dist := (kxy - minxy) / maxxy
	return dist, nil
}

func complexity(cacher map[string]float64, intelligence, x string) (float64, error) {
	switch intelligence {
	case "ctw":
		return complexityCTW(cacher, x)
	default:
		return complexityTarGz(x)
	}
}

func complexityCTW(cacher map[string]float64, fpath string) (float64, error) {
	size, ok := cacher[fpath]
	if ok {
		return size, nil
	}

	buf := bytes.NewBuffer(nil)
	if err := ctw.Compress(buf, fpath, 48); err != nil {
		return -1, errors.Wrap(err, "")
	}
	size = float64(buf.Len())

	cacher[fpath] = size
	return size, nil
}

func complexityTarGz(fpath string) (float64, error) {
	dst := "/tmp/dst"
	if err := exec.Command("tar", "zcf", dst, fpath).Run(); err != nil {
		return -1, errors.Wrap(err, "")
	}
	info, err := os.Stat(dst)
	if err != nil {
		return -1, errors.Wrap(err, "")
	}
	return float64(info.Size()), nil
}

func concatFiles(tmpf *os.File, fs ...string) error {
	for _, fpath := range fs {
		err := func(fpath string) error {
			f, err := os.Open(fpath)
			if err != nil {
				return errors.Wrap(err, "")
			}
			defer f.Close()
			if _, err := io.Copy(tmpf, f); err != nil {
				return errors.Wrap(err, "")
			}
			return nil
		}(fpath)
		if err != nil {
			return errors.Wrap(err, "")
		}
	}
	if err := tmpf.Close(); err != nil {
		return errors.Wrap(err, "")
	}
	return nil
}

func distanceMatrix(intelligence string, data []string) ([]float64, error) {
	cacher := make(map[string]float64)

	n := len(data)
	mat := make([]float64, 0, n*(n-1)/2)
	for i, dx := range data[:n-1] {
		for _, dy := range data[i+1:] {
			dist, err := distance(cacher, intelligence, dx, dy)
			if err != nil {
				return nil, errors.Wrap(err, "")
			}
			mat = append(mat, dist)
			log.Printf("\"%s\"-\"%s\": %f", dx, dy, dist)
		}
	}
	return mat, nil
}

func listFiles(dir string) ([]string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}
	data := make([]string, 0, len(files))
	for _, f := range files {
		fpath := filepath.Join(dir, f.Name())
		data = append(data, fpath)
	}
	return data, nil
}
