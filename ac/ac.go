// Package ac defines the interfaces the arithmetic coding algorithm requires.
// See its subpackages for particular finite precision realizations of the algorithm.
package ac

import (
	"fmt"
)

// ErrDecodeInsufficientBits is returned when there are insufficient bits sent to Decode to reconstruct the original data.
var ErrDecodeInsufficientBits = fmt.Errorf("insufficient bits sent to decoder")

// A Model is a probabilistic model on a sequence of binary data,
// as expected by the arithmetic coding algorithm.
type Model interface {
	// Prob0 returns the probability that the next bit will be zero.
	Prob0() float64

	// Observe informs the Model that a bit is observed from the sequence.
	Observe(bit int)
}
