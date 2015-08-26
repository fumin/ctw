package ctw

import (
	"math"
	"testing"
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
