package main

import (
	"flag"
	"log"
	"os"

	"github.com/fumin/ctw"
)

var depth = flag.Int("depth", 48, "depth of Context Tree Weighting")

func main() {
	flag.Parse()
	if err := ctw.Decompress(os.Stdout, os.Stdin, *depth); err != nil {
		log.Fatalf("%v", err)
	}
}
