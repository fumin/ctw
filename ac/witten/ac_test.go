package witten

import (
	"io/ioutil"
	"sync"
	"testing"
)

func TestEncodeConstModel(t *testing.T) {
	model := func(p float64) func() Model {
		return func() Model {
			return &ConstModel{P0: p}
		}
	}

	testEncode(t, model(0.75))

	// Test the edge case where v_0 == (1 << (f+1))
	testEncode(t, model(0.5))

	// Test the case where the probability of zero is less than 0.5.
	// The description of the algorithm assumes that the probability of zero is larger than 0.5,
	// and explains that the opposite case can be handled by relabeling the zeros and ones.
	// We check that the implementation conducts such relabelings correctly.
	testEncode(t, model(0.25))

	// Test that the "creating zeros in delay register" mechanism is working
	testEncode(t, model(0.000000025))
}

func testEncode(t *testing.T, model func() Model) {
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

	Encode(dst, src, model())
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

	if err := Decode(ddst, dsrc, model(), int64(len(x))); err != nil {
		t.Fatalf("%+v", err)
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

type ConstModel struct {
	P0 float64
}

func (m *ConstModel) Prob0() float64 {
	return m.P0
}

func (m *ConstModel) Observe(b int) {}
