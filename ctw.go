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

// Node represents a suffix in a Context Tree Weighting.
// It holds the log probability of the source sequence given the suffix represented by the node.
type Node struct {
	LogProb float64 // log probability of suffix

	a    uint32  // number of zeros with suffix
	b    uint32  // number of ones with suffix
	lktp float64 // log probability of the Krichevsky-Trofimov (KT) Estimation, given our current number of zeros and ones.

	left  *Node // the sub-suffix that ends with one
	right *Node // the sub-suffix that ends with zero
}

// SeqProb returns the probability of a sequence if it is followed by a bit.
// Argument root is the root of the context tree.
// Argument bits is the last few bits of the sequence, len(bits) should be the depth of the tree.
// Argument bit is the new bit following the sequence.
// Argument update determines whether the tree is updated after the calculation.
// If update is false, the changes required by the calculation are rollbacked, and the tree remains unchanged.
func SeqProb(root *Node, bits []int, bit int, update bool) float64 {
	if bit != 0 && bit != 1 {
		log.Fatalf("wrong bit %d", bit)
	}

	// Update the counts of zeros and ones of each node.
	type Snapshot struct {
		Node  *Node
		State Node
		IsNew bool
	}
	traversed := []Snapshot{}
	node := root
	traversed = append(traversed, Snapshot{Node: node, State: *node, IsNew: false})
	krichevskyTrofimov(node, bit)

	for d := 0; d < len(bits); d++ {
		isNew := false
		if bits[len(bits)-1-d] == 0 {
			if node.right == nil {
				node.right = &Node{}
				isNew = true
			}
			node = node.right
		} else {
			if node.left == nil {
				node.left = &Node{}
				isNew = true
			}
			node = node.left
		}

		traversed = append(traversed, Snapshot{Node: node, State: *node, IsNew: isNew})
		krichevskyTrofimov(node, bit)
	}

	// Update the actual node probabilities.
	for i := len(traversed) - 1; i >= 0; i-- {
		ss := traversed[i]
		node := ss.Node

		if node.left != nil && node.right != nil {
			node.LogProb = math.Log(0.5) + logaddexp(node.lktp, node.left.LogProb+node.right.LogProb)
		} else {
			node.LogProb = node.lktp
		}
	}
	seqProb := root.LogProb

	// Rollback changes to the tree if necessary.
	if !update {
		for i, ss := range traversed {
			node := ss.Node
			node.lktp = ss.State.lktp
			node.a = ss.State.a
			node.b = ss.State.b
			node.LogProb = ss.State.LogProb

			if i < len(traversed)-2 {
				next := traversed[i+1]
				if next.IsNew {
					if next.Node == node.right {
						node.right = nil
					} else {
						node.left = nil
					}
					break
				}
			}
		}
	}

	return seqProb
}

// krichevskyTrofimov updates the Krichevsky-Trofimov estimate of a node given a new observed bit.
func krichevskyTrofimov(node *Node, bit int) {
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
// CTW implements the Model interface.
type CTW struct {
	bits []int
	root *Node
}

// NewCTW returns a new CTW whose context tree's depth is len(bits).
// The prior context of the tree is given by bits.
func NewCTW(bits []int) *CTW {
	model := &CTW{
		bits: bits,
		root: &Node{},
	}
	return model
}

// Prob0 returns the probability that the next bit be zero.
func (model *CTW) Prob0() float64 {
	joint0 := SeqProb(model.root, model.bits, 0, false)
	joint1 := SeqProb(model.root, model.bits, 1, false)
	cond := 1.0 / (1 + math.Exp(joint1-joint0))
	return cond
}

// Observe updates the context tree, given that the sequence is followed by bit.
func (model *CTW) Observe(bit int) {
	SeqProb(model.root, model.bits, bit, true)
	for i := 1; i < len(model.bits); i++ {
		model.bits[i-1] = model.bits[i]
	}
	model.bits[len(model.bits)-1] = bit
}
