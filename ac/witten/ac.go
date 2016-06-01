// Package witten implements the arithmetic coding algorithm described in
// Witten, Ian H.; Neal, Radford M.; Cleary, John G. (June 1987). "Arithmetic Coding for Data Compression". Communications of the ACM 30 (6): 520–540.
package witten

import (
	"fmt"
)

const (
	codeValueBits = 32
	topValue      = (uint64(1) << codeValueBits) - 1
	firstQtr      = topValue/4 + 1
	half          = 2 * firstQtr
	thirdQtr      = 3 * firstQtr

	topValueDbl = float64(topValue)
)

// An arithmeticEncoder carries the state required by an encoder.
type arithmeticEncoder struct {
	low   uint64
	high  uint64
	fbits uint64
}

func newAE() *arithmeticEncoder {
	ae := &arithmeticEncoder{}
	ae.high = topValue
	return ae
}

func bitPlusFollow(dst chan<- int, ae *arithmeticEncoder, bit int) {
	negbit := 0
	if bit == 0 {
		negbit = 1
	}

	dst <- bit
	for ae.fbits > 0 {
		dst <- negbit
		ae.fbits -= 1
	}
}

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
	ae := newAE()
	for bit := range src {
		prob0 := model.Prob0()
		model.Observe(bit)

		arange := (ae.high - ae.low) + 1
		split := ae.low + arange*uint64(prob0*topValueDbl)/topValue

		// narrow range
		if bit == 1 {
			ae.low = split
		} else {
			ae.high = split - 1
		}

		for {
			if ae.high < half {
				bitPlusFollow(dst, ae, 0)
			} else if ae.low >= half {
				bitPlusFollow(dst, ae, 1)
				ae.low -= half
				ae.high -= half
			} else if ae.low >= firstQtr && ae.high < thirdQtr {
				ae.fbits += 1
				ae.low -= firstQtr
				ae.high -= firstQtr
			} else {
				break
			}

			ae.low = 2 * ae.low
			ae.high = 2*ae.high + 1
		}

	}

	ae.fbits += 1
	if ae.low < firstQtr {
		bitPlusFollow(dst, ae, 0)
	} else {
		bitPlusFollow(dst, ae, 1)
	}
}

type arithmeticDecoder struct {
	low   uint64
	high  uint64
	value uint64
}

func newAD() *arithmeticDecoder {
	ad := &arithmeticDecoder{}
	ad.high = topValue
	return ad
}

// ErrDecodeInsufficientBits is returned when there are insufficient bits sent to Decode to reconstruct the original data.
var ErrDecodeInsufficientBits = fmt.Errorf("insufficient bits sent to decoder")

func Decode(dst chan<- int, src <-chan int, model Model, originalSize int64) error {
	defer close(dst)

	garbageBits := 0
	readDecBit := func(src <-chan int) (uint64, error) {
		b, ok := <-src
		if ok {
			return uint64(b), nil
		}
		garbageBits++
		if garbageBits > codeValueBits-2 {
			return 0, ErrDecodeInsufficientBits
		}
		return 1, nil // the returned bit can actually be random
	}

	ad := newAD()
	for i := 1; i <= codeValueBits; i++ {
		inb, err := readDecBit(src)
		if err != nil {
			return err
		}
		ad.value = 2*ad.value + inb
	}

	for i := int64(0); i < originalSize; i++ {
		prob0 := model.Prob0()

		arange := (ad.high - ad.low) + 1
		split := ad.low + arange*uint64(prob0*topValueDbl)/topValue

		bit := 1
		if ad.value < split {
			bit = 0
		}
		dst <- bit
		model.Observe(bit)

		// narrow range
		if bit == 1 {
			ad.low = split
		} else {
			ad.high = split - 1
		}

		// rescale interval
		for {

			if ad.high < half {
				// do nothing
			} else if ad.low >= half {
				ad.value -= half
				ad.low -= half
				ad.high -= half
			} else if ad.low >= firstQtr && ad.high < thirdQtr {
				ad.value -= firstQtr
				ad.low -= firstQtr
				ad.high -= firstQtr
			} else {
				break
			}

			ad.low = 2 * ad.low
			ad.high = 2*ad.high + 1
			inb, err := readDecBit(src)
			if err != nil {
				return err
			}
			ad.value = 2*ad.value + inb
		}
	}
	return nil
}
