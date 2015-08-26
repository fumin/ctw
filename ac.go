package ctw

import (
	"fmt"
	"math"
)

const (
	// f is the precision in bits we assume our probability models are operating in.
	// In particular, the accumulator is assumed to be f+1 binary digits wide.
	//
	// We need to be aware that it cannot be too big, for a larger f implies a higher floating point precision requirement when calculating the exp-tables A and B.
	// In particular, a useful check is that A[1] should give ((1 << f) - 1), not (1 << f).
	f uint = 20

	// d is the width of the delay register in bits.
	// Here, since we are using an uint64 for the delay register, d is set to 64.
	d uint = 64
)

// A Model is a probabilistic model on a sequence of binary data.
type Model interface {
	// Prob0 returns the probability that the next bit will be zero.
	Prob0() float64

	// Observe informs the Model that a bit is observed from the sequence.
	Observe(bit int)
}

// Encode performs arithmetic coding on a stream of bits given a binary probabilistic model.
// The input bits should be sent through src, which Encode consumes until it is closed.
// The output bits can be received from dst. Encode will block when dst if full and is not read from.
// Encode closes dst when the encoding is complete and there are no more bits to be sent to it.
func Encode(dst chan<- int, src <-chan int, model Model) {
	defer close(dst)
	var dlreg uint64 = 0
	var accum uint64 = 0
	var v uint64 = 1
	A, B := expTables()

	for x := range src {
		// Prepare v_0 and xt
		prob0 := model.Prob0()
		model.Observe(x)
		var p float64
		var xt int
		if prob0 > 0.5 {
			p = prob0
			xt = x
		} else {
			p = 1 - prob0
			if x == 1 {
				xt = 0
			} else {
				xt = 1
			}
		}
		v_0 := uint64(math.Exp2(float64(f))*math.Log2(1/p) + 0.5)
		if v_0 < 3 {
			v_0 = 3
		}

		// Scaling and pushing
		for v > (1 << f) {
			if dlreg >= (1 << (d - 1)) {
				dst <- 1
				dlreg = 2 * (dlreg - (1 << (d - 1)))
			} else {
				dst <- 0
				dlreg = 2 * dlreg
			}

			if accum >= (1 << f) {
				dlreg = dlreg + 1
				accum = 2 * (accum - (1 << f))
			} else {
				accum = 2 * accum
			}

			v = v - (1 << f)
		}

		// Creating zeors in delay register
		for dlreg == ((1 << d) - 1) {
			dst <- 1
			dlreg = 2 * (dlreg - (1 << (d - 1)))

			if accum >= (1 << f) {
				dlreg = dlreg + 1
				accum = 2 * (accum - (1 << f))
			} else {
				accum = 2 * accum
			}
		}

		v0 := v + v_0
		if xt == 1 {
			if v0 <= (1 << f) {
				accum = accum + 2*A[v0]
				if accum >= (1 << (f + 1)) {
					dlreg = dlreg + 1
					accum = accum - (1 << (f + 1))
				}
				v = B[A[v]-A[v0]]
			} else {
				accum = accum + A[v0-(1<<f)]
				if accum >= (1 << (f + 1)) {
					dlreg = dlreg + 1
					accum = accum - (1 << (f + 1))
				}
				v = B[2*A[v]-A[v0-(1<<f)]] + (1 << f)
			}
		} else {
			v = v0
		}
	}

	// Termination
	for i := 1; i <= int(d); i++ {
		if dlreg < (1 << (d - 1)) {
			dst <- 0
			dlreg = dlreg * 2
		} else {
			dst <- 1
			dlreg = (dlreg - (1 << (d - 1))) * 2
		}
	}
	for i := 1; i <= int(f+1); i++ {
		if accum < (1 << f) {
			dst <- 0
			accum = accum * 2
		} else {
			dst <- 1
			accum = (accum - (1 << f)) * 2
		}
	}
}

// ErrDecodeInsufficientBits is returned when there are insufficient bits sent to Decode to reconstruct the original data.
var ErrDecodeInsufficientBits = fmt.Errorf("insufficient bits sent to decoder")

// Decode decodes a stream of bits encoded by Encode using arithmetic coding.
//
// Completion of the decoding is determined by originalSize, which is the number of bits of the original data before encoding.
// Decode consumes bits from src until either it is closed, or when it has decoded originalSize number of bits.
// Therefore, it is important that callers close src when there are no more bits to be sent to Decode, so that Decode will not block indefinitely on receiving from src.
// Equally important is that callers do not block indefinitely when sending to src, since Decode can return early and stop consuming src anytime.
// A typical pattern is for callers to multiplex on src and a separate channel that is signaled when the decoding completes or errors.
//
// The output decoded bits can be received from dst. Decode will block when dst is full and is not read from.
// Decode closes dst when the decoding is complete.
// Decode expects that model is the exact same probabilistic model used in Encode.
// ErrDecodeInsufficientBits is returned if src is closed before originalSize number of bits have been decoded.
func Decode(dst chan<- int, src <-chan int, model Model, originalSize int64) error {
	defer close(dst)
	var dlreg uint64 = 0
	var accum uint64 = 0
	var v uint64 = 1
	var cdlreg uint64 = 0
	var caccum uint64 = 0
	for i := 1; i <= int(d); i++ {
		pull, ok := <-src
		if !ok {
			return ErrDecodeInsufficientBits
		}
		cdlreg = cdlreg*2 + uint64(pull)
	}
	for i := 1; i <= int(f+1); i++ {
		pull, ok := <-src
		if !ok {
			return ErrDecodeInsufficientBits
		}
		caccum = caccum*2 + uint64(pull)
	}
	A, B := expTables()

	for i := int64(0); i < originalSize; i++ {
		// Prepare v_0
		prob0 := model.Prob0()
		var p float64
		if prob0 > 0.5 {
			p = prob0
		} else {
			p = 1 - prob0
		}
		v_0 := uint64(math.Exp2(float64(f))*math.Log2(1/p) + 0.5)
		if v_0 < 3 {
			v_0 = 3
		}

		// Scaling and pulling
		for v > (1 << f) {
			if dlreg >= (1 << (d - 1)) {
				dlreg = 2 * (dlreg - (1 << (d - 1)))
			} else {
				dlreg = 2 * dlreg
			}
			if accum >= (1 << f) {
				dlreg = dlreg + 1
				accum = 2 * (accum - (1 << f))
			} else {
				accum = 2 * accum
			}
			v = v - (1 << f)
			if cdlreg >= (1 << (d - 1)) {
				cdlreg = 2 * (cdlreg - (1 << (d - 1)))
			} else {
				cdlreg = 2 * cdlreg
			}

			pl, ok := <-src
			if !ok {
				return ErrDecodeInsufficientBits
			}
			if caccum >= (1 << f) {
				cdlreg = cdlreg + 1
				caccum = 2*(caccum-(1<<f)) + uint64(pl)
			} else {
				caccum = 2*caccum + uint64(pl)
			}
		}

		// Creating zeros in delay register
		for dlreg == ((1 << d) - 1) {
			dlreg = 2 * (dlreg - (1 << (d - 1)))
			if accum >= (1 << f) {
				dlreg = dlreg + 1
				accum = 2 * (accum - (1 << f))
			} else {
				accum = 2 * accum
			}
			if cdlreg >= (1 << (d - 1)) {
				cdlreg = 2 * (cdlreg - (1 << (d - 1)))
			} else {
				cdlreg = 2 * cdlreg
			}

			pl, ok := <-src
			if !ok {
				return ErrDecodeInsufficientBits
			}
			if caccum >= (1 << f) {
				cdlreg = cdlreg + 1
				caccum = 2*(caccum-(1<<f)) + uint64(pl)
			} else {
				caccum = 2*caccum + uint64(pl)
			}
		}

		// Adding A[v0] to the accumulator (or not) and computing v.
		// At the same time, decode the next bit xt.
		var xt int
		v0 := v + v_0
		if v0 <= (1 << f) {
			taccum := accum + 2*A[v0]
			tdlreg := dlreg
			if taccum >= (1 << (f + 1)) {
				tdlreg = tdlreg + 1
				taccum = taccum - (1 << (f + 1))
			}
			if (cdlreg == tdlreg && caccum < taccum) || (cdlreg < tdlreg) {
				xt = 0
			} else {
				xt = 1
			}
			if xt == 1 {
				accum = taccum
				dlreg = tdlreg
				v = B[A[v]-A[v0]]
			} else {
				v = v0
			}
		} else {
			taccum := accum + A[v0-(1<<f)]
			tdlreg := dlreg
			if taccum >= (1 << (f + 1)) {
				tdlreg = tdlreg + 1
				taccum = taccum - (1 << (f + 1))
			}
			if (cdlreg == tdlreg && caccum < taccum) || (cdlreg < tdlreg) {
				xt = 0
			} else {
				xt = 1
			}
			if xt == 1 {
				accum = taccum
				dlreg = tdlreg
				v = B[2*A[v]-A[v0-(1<<f)]] + (1 << f)
			} else {
				v = v0
			}
		}

		// Handle relabeling and output decoded bit.
		if prob0 <= 0.5 {
			if xt == 0 {
				xt = 1
			} else {
				xt = 0
			}
		}
		model.Observe(xt)
		dst <- xt
	}

	return nil
}

// expTables prepares the exp-tables described in section 6.4 of the EIDMA report by F.M.J. Willems and Tj. J. Tjalkens.
// Complexity Reduction of the Context-Tree Weighting Algorithm: A Study for KPN Research, Technical University of Eindhoven, EIDMA Report RS.97.01
func expTables() ([]uint64, []uint64) {
	var pow2f float64 = 1 << f
	A := make([]uint64, int(pow2f)+1)
	for i := 1; i <= int(pow2f); i++ {
		A[i] = uint64(pow2f*math.Exp2(-float64(i)/pow2f) + 0.5)
	}

	// B entries for (1<<(f-1)), (1<<f)-1
	B := make([]uint64, int(pow2f))
	for j := 1 << (f - 1); j <= (1<<f)-1; j++ {
		B[j] = uint64(-pow2f*math.Log2(float64(j)/pow2f) + 0.5)
	}

	// B entries for 1,(1<<(f-1))-1
	for j := 1; j < (1 << (f - 1)); j++ {
		k := math.Ceil(float64(f) - 1 - math.Log2(float64(j)))
		b2kj := B[int(math.Exp2(k))*j]
		if b2kj == 0 {
			panic("")
		}
		B[j] = b2kj + uint64(k*pow2f)
	}

	return A, B
}
