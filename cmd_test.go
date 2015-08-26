package ctw

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestCompress(t *testing.T) {
	const name = "gettysburg.txt"
	const depth = 48

	// Compress
	f, err := ioutil.TempFile("", "ctw.TestCompress.Compress")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer f.Close()
	defer os.Remove(f.Name())
	if err := Compress(f, name, depth); err != nil {
		t.Fatalf("%v", err)
	}

	// Decompress
	_, err = f.Seek(0, 0)
	if err != nil {
		t.Fatalf("%v", err)
	}
	df, err := ioutil.TempFile("", "ctw.TestCompress.Decompress")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer df.Close()
	defer os.Remove(df.Name())
	if err := Decompress(df, f, depth); err != nil {
		t.Fatalf("%v", err)
	}

	// Check if the decompressed result is the same as the original file
	_, err = df.Seek(0, 0)
	if err != nil {
		t.Fatalf("%v", err)
	}
	decom, err := ioutil.ReadAll(df)
	if err != nil {
		t.Fatalf("%v", err)
	}
	gettys, err := ioutil.ReadFile(name)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !bytes.Equal(gettys, decom) {
		t.Errorf("%v %v", gettys, decom)
	}
}
