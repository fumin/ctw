package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fumin/ctw"
)

var depth = flag.Int("depth", 48, "depth of Context Tree Weighting")
var verbose = flag.Bool("verbose", false, "verbosity")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] filename\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	name := flag.Arg(0)
	if name == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := ctw.Compress(os.Stdout, name, *depth); err != nil {
		log.Fatalf("%v", err)
	}
}
