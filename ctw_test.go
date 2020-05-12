package ctw

import (
	"flag"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sync"
	"testing"

	"github.com/fumin/ctw/ac/witten"
)

// TestUpdateSunehag tests the examples in the slides by Peter Sunehag and Marcus Hutter
// http://cs.anu.edu.au/courses/COMP4620/2013/slides-ctw.pdf
func TestSunehag(t *testing.T) {
	t.Parallel()
	root := &treeNode{}
	depth := 3
	bits := []int{1, 1, 0}

	source := []int{0, 1, 0, 0, 1, 1, 0}
	for _, b := range source {
		update(root, bits[len(bits)-depth:], b)
		bits = append(bits, b)
	}
	if math.Abs(root.LogProb-math.Log(7.0/2048)) > 1e-8 {
		t.Errorf("%f", root.LogProb)
	}

	b := 0
	update(root, bits[len(bits)-depth:], b)
	bits = append(bits, b)
	if math.Abs(root.LogProb-math.Log(153.0/65536)) > 1e-8 {
		log.Printf("%f", root.LogProb)
		t.Errorf("%f", root.LogProb)
	}
}

// TestUpdateEIDMA tests the examle in the EIDMA report by F.M.J. Willems and Tj. J. Tjalkens.
// Complexity Reduction of the Context-Tree Weighting Algorithm: A Study for KPN Research, Technical University of Eindhoven, EIDMA Report RS.97.01
func TestEIDMA(t *testing.T) {
	t.Parallel()
	root := &treeNode{}
	depth := 3
	bits := []int{0, 1, 0}

	source := []int{0, 1, 1, 0, 1, 0, 0}
	for _, b := range source {
		update(root, bits[len(bits)-depth:], b)
		bits = append(bits, b)
	}
	if math.Abs(root.LogProb-math.Log(95.0/32768)) > 1e-8 {
		t.Errorf("%f", root.LogProb)
	}
}

// TestRevert tests that revert reverts the state of the tree to its original one.
func TestRevert(t *testing.T) {
	t.Parallel()
	root := &treeNode{}
	depth := 3
	bits := []int{0, 1, 0}

	source := []int{0, 1, 1, 0, 1, 0, 0}
	for _, b := range source {
		traversal := update(root, bits[len(bits)-depth:], b)
		seqP := root.LogProb

		revert(traversal)

		update(root, bits[len(bits)-depth:], b)
		seqPAfter := root.LogProb

		if seqP != seqPAfter {
			t.Errorf("%f %f", seqP, seqPAfter)
		}

		bits = append(bits, b)
	}
}

func TestCTWReverter(t *testing.T) {
	t.Parallel()
	model := NewCTW(make([]int, 48))
	x := []int{1, 1, 0, 1, 0, 0, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1, 1, 1, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0}
	for _, xi := range x {
		model.Observe(xi)
	}
	prob0 := model.Prob0()

	reverter := NewCTWReverter(model)
	y := []int{0, 1, 0, 0, 1, 1, 1, 0, 1, 0, 1, 1, 0}
	for _, yi := range y {
		reverter.Observe(yi)
		reverter.Unobserve()

		reverter.Observe(yi)
		reverter.Observe(yi)
		reverter.Unobserve()
	}
	prob0Updated := model.Prob0()
	if prob0Updated == prob0 {
		t.Fatalf("%f %f", prob0Updated, prob0)
	}

	for _, _ = range y {
		reverter.Unobserve()
	}

	prob0Reverted := model.Prob0()
	if prob0Reverted != prob0 {
		t.Fatalf("%f %f", prob0Reverted, prob0)
	}
}

func TestEncode(t *testing.T) {
	t.Parallel()
	// Prepare data
	// x := []int{1, 1, 0, 1, 0, 0, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1, 1, 1, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0}
	contents, err := ioutil.ReadFile("gettysburg.txt")
	if err != nil {
		t.Fatalf("%v", err)
	}
	x := []int{}
	for _, bt := range contents {
		for i := uint(0); i < 8; i++ {
			x = append(x, int(bt)&(1<<i)>>i)
		}
	}

	// Encode
	src := make(chan int)
	go func() {
		for _, b := range x {
			src <- b
		}
		close(src)
	}()

	encoded := []int{}
	dst := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for b := range dst {
			encoded = append(encoded, b)
		}
	}()

	witten.Encode(dst, src, NewCTW(make([]int, 48)))
	wg.Wait()
	// t.Logf("encoded bits: %d, original bits: %d", len(encoded), len(x))

	// Decode
	dsrc := make(chan int)
	go func() {
		for i := range encoded {
			dsrc <- encoded[i]
		}
		close(dsrc)
	}()

	decoded := []int{}
	ddst := make(chan int)
	wg = sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for b := range ddst {
			decoded = append(decoded, b)
		}
	}()

	if err := witten.Decode(ddst, dsrc, NewCTW(make([]int, 48)), int64(len(x))); err != nil {
		t.Fatalf("%v", err)
	}
	wg.Wait()

	// Check that the decoded result is correct.
	if len(x) != len(decoded) {
		t.Fatalf("%d != %d", len(x), len(decoded))
	}
	for i, b := range x {
		if decoded[i] != b {
			t.Errorf("%d: %d != %d", i, b, decoded[i])
		}
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	os.Exit(m.Run())
}
