package ctw

import (
	"io/ioutil"
	"math"
	"sync"
	"testing"

	"github.com/fumin/ctw/ac/witten"
)

// TestUpdateSunehag tests the examples in the slides by Peter Sunehag and Marcus Hutter
// http://cs.anu.edu.au/courses/COMP4620/2013/slides-ctw.pdf
func TestSunehag(t *testing.T) {
	root := &Node{}
	depth := 3
	bits := []int{1, 1, 0}

	source := []int{0, 1, 0, 0, 1, 1, 0}
	for _, b := range source {
		SeqProb(root, bits[len(bits)-depth:], b, true)
		bits = append(bits, b)
	}
	if math.Abs(root.LogProb-math.Log(7.0/2048)) > 1e-8 {
		t.Errorf("%f", root.LogProb)
	}

	b := 0
	SeqProb(root, bits[len(bits)-depth:], b, true)
	bits = append(bits, b)
	if math.Abs(root.LogProb-math.Log(153.0/65536)) > 1e-8 {
		t.Errorf("%f", root.LogProb)
	}
}

// TestUpdateEIDMA tests the examle in the EIDMA report by F.M.J. Willems and Tj. J. Tjalkens.
// Complexity Reduction of the Context-Tree Weighting Algorithm: A Study for KPN Research, Technical University of Eindhoven, EIDMA Report RS.97.01
func TestEIDMA(t *testing.T) {
	root := &Node{}
	depth := 3
	bits := []int{0, 1, 0}

	source := []int{0, 1, 1, 0, 1, 0, 0}
	for _, b := range source {
		SeqProb(root, bits[len(bits)-depth:], b, true)
		bits = append(bits, b)
	}
	if math.Abs(root.LogProb-math.Log(95.0/32768)) > 1e-8 {
		t.Errorf("%f", root.LogProb)
	}
}

// TestSeqProbNoUpdate tests that SeqProb called with update false does not update the context tree,
// and that the returned value is the same as the root's probability after update.
func TestSeqProbNoUpdate(t *testing.T) {
	root := &Node{}
	depth := 3
	bits := []int{0, 1, 0}

	source := []int{0, 1, 1, 0, 1, 0, 0}
	for _, b := range source {
		seqP := SeqProb(root, bits[len(bits)-depth:], b, false)
		SeqProb(root, bits[len(bits)-depth:], b, true)

		if seqP != root.LogProb {
			t.Errorf("%f", seqP)
		}

		bits = append(bits, b)
	}
}

func TestEncode(t *testing.T) {
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
	t.Logf("encoded bits: %d, original bits: %d", len(encoded), len(x))

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
