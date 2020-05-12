// Package ctw provides an implementation of the Context Tree Weighting algorithm.
// Also contained is an implementation of the Rissanen-Langdon Arithmetic Coding algorithm, which is combined with Context Tree Weighting to create a lossless compression/decompression utility.
//
// Below is an example of using this package to compress Lincoln's Gettysburg address:
//    go run compress/main.go gettysburg.txt > gettys.ctw
//    cat gettys.ctw | go run decompress/main.go > gettys.dctw
//    diff gettysburg.txt gettys.dctw
//
// Reference:
// F.M.J. Willems and Tj. J. Tjalkens, Complexity Reduction of the Context-Tree Weighting Algorithm: A Study for KPN Research, Technical University of Eindhoven, EIDMA Report RS.97.01.
package ctw

import (
	"log"
	"math"
)

// logaddexp performs log(exp(x) + exp(y))
func logaddexp(x, y float64) float64 {
	tmp := x - y
	if tmp > 0 {
		return x + math.Log1p(math.Exp(-tmp))
	} else if tmp <= 0 {
		return y + math.Log1p(math.Exp(tmp))
	} else {
		// Nans, or infinities of the same sign involved
		log.Printf("logaddexp %f %f", x, y)
		return x + y
	}
}

// treeNode represents a suffix in a Context Tree Weighting.
// It holds the log probability of the source sequence given the suffix represented by the node.
type treeNode struct {
	LogProb float64 // log probability of suffix

	a    uint32  // number of zeros with suffix
	b    uint32  // number of ones with suffix
	lktp float64 // log probability of the Krichevsky-Trofimov (KT) Estimation, given our current number of zeros and ones.

	left  *treeNode // the sub-suffix that ends with one
	right *treeNode // the sub-suffix that ends with zero
}

type snapshot struct {
	node  *treeNode
	state treeNode
	isNew bool
}

func revert(traversed []snapshot) {
	for _, ss := range traversed {
		node := ss.node
		node.lktp = ss.state.lktp
		node.a = ss.state.a
		node.b = ss.state.b
		node.LogProb = ss.state.LogProb

		// The memory releasing logic below saves memory.
		// However, it might increase GC times if the released memory is added back again.
		// This happens when our predictions are faily consistent with the eventually arriving data.
		// Here we emphasize performance by not doing this memory saving optimization.
		//
		// if i < len(traversed)-2 {
		// 	next := traversed[i+1]
		// 	if next.IsNew {
		// 		if next.Node == node.right {
		// 			node.right = nil
		// 		} else {
		// 			node.left = nil
		// 		}
		// 		break
		// 	}
		// }
	}
}

// update updates the tree according to the rules of CTW.
// Root is the root of the context tree.
// Bits is the last few bits of the sequence, len(bits) should be the depth of the tree.
// Bit is the new bit following the sequence.
func update(root *treeNode, bits []int, bit int) []snapshot {
	if bit != 0 && bit != 1 {
		log.Fatalf("wrong bit %d", bit)
	}

	// Update the counts of zeros and ones of each node.
	traversed := []snapshot{}
	node := root
	traversed = append(traversed, snapshot{node: node, state: *node, isNew: false})
	krichevskyTrofimov(node, bit)

	for d := 0; d < len(bits); d++ {
		isNew := false
		if bits[len(bits)-1-d] == 0 {
			if node.right == nil {
				node.right = &treeNode{}
				isNew = true
			}
			node = node.right
		} else {
			if node.left == nil {
				node.left = &treeNode{}
				isNew = true
			}
			node = node.left
		}

		traversed = append(traversed, snapshot{node: node, state: *node, isNew: isNew})
		krichevskyTrofimov(node, bit)
	}

	// Update the actual node probabilities.
	for i := len(traversed) - 1; i >= 0; i-- {
		ss := traversed[i]
		node := ss.node

		if node.left != nil || node.right != nil {
			var lp float64 = 0
			if node.left != nil {
				lp = node.left.LogProb
			}
			var rp float64 = 0
			if node.right != nil {
				rp = node.right.LogProb
			}
			w := 0.5
			node.LogProb = logaddexp(math.Log(w)+node.lktp, math.Log(1-w)+lp+rp)
		} else {
			node.LogProb = node.lktp
		}
	}

	return traversed
}

// krichevskyTrofimov updates the Krichevsky-Trofimov estimate of a node given a new observed bit.
func krichevskyTrofimov(node *treeNode, bit int) {
	a := float64(node.a)
	b := float64(node.b)
	if bit == 0 {
		node.lktp = node.lktp + math.Log(a+0.5) - math.Log(a+b+1)
		node.a += 1
	} else {
		node.lktp = node.lktp + math.Log(b+0.5) - math.Log(a+b+1)
		node.b += 1
	}
}

// A CTW is a Context Tree Weighting based probabilistic model for binary data.
// CTW implements the arithmetic coding Model interface.
type CTW struct {
	bits []int
	root *treeNode
}

// NewCTW returns a new CTW whose context tree's depth is len(bits).
// The prior context of the tree is given by bits.
func NewCTW(bits []int) *CTW {
	model := &CTW{
		bits: bits,
		root: &treeNode{},
	}
	return model
}

// Prob0 returns the probability that the next bit be zero.
func (model *CTW) Prob0() float64 {
	before := model.root.LogProb
	traversal := update(model.root, model.bits, 0)
	after := model.root.LogProb

	revert(traversal)

	return math.Exp(after - before)
}

// Observe updates the context tree, given that the sequence is followed by bit.
func (model *CTW) Observe(bit int) {
	model.observe(bit)
}

func (model *CTW) observe(bit int) []snapshot {
	traversal := update(model.root, model.bits, bit)
	for i := 1; i < len(model.bits); i++ {
		model.bits[i-1] = model.bits[i]
	}
	model.bits[len(model.bits)-1] = bit
	return traversal
}

// A CTWReverter is a CTW model that allows reverting to its previous state.
// This is useful for predicting several steps ahead, while keeping the model's original state intact.
type CTWReverter struct {
	model      *CTW
	bits       []int
	traversals [][]snapshot
}

func NewCTWReverter(model *CTW) *CTWReverter {
	cr := &CTWReverter{}
	cr.model = model
	return cr
}

func (cr *CTWReverter) Prob0() float64 {
	return cr.model.Prob0()
}

func (cr *CTWReverter) Observe(bit int) {
	cr.bits = append(cr.bits, cr.model.bits[0])
	cr.traversals = append(cr.traversals, cr.model.observe(bit))
}

func (cr *CTWReverter) Unobserve() {
	// Revert the tree.
	tvIdx := len(cr.traversals) - 1
	revert(cr.traversals[tvIdx])
	cr.traversals = cr.traversals[:tvIdx]

	// Revert the context bits.
	for i := len(cr.model.bits) - 1; i > 0; i-- {
		cr.model.bits[i] = cr.model.bits[i-1]
	}
	btIdx := len(cr.bits) - 1
	cr.model.bits[0] = cr.bits[btIdx]
	cr.bits = cr.bits[:btIdx]
}
