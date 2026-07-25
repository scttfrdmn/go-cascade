package solution

import "testing"

func TestVBasic(t *testing.T) {
	cases := []struct {
		in   uint64
		want int
	}{
		{1, 0},
		{2, 1},
		{3, 7},
		{4, 2},
		{5, 5},
		{6, 8},
		{7, 16},
		{8, 3},
		{16, 4},
		{27, 111}, // classic long trajectory
	}
	for _, c := range cases {
		if got := CollatzSteps(c.in); got != c.want {
			t.Errorf("CollatzSteps(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestVPowersOfTwo(t *testing.T) {
	// A power of two 2^k reaches 1 in exactly k halving steps.
	var n uint64 = 1
	for k := 0; k < 30; k++ {
		if got := CollatzSteps(n); got != k {
			t.Errorf("CollatzSteps(2^%d=%d) = %d, want %d", k, n, got, k)
		}
		n *= 2
	}
}

func TestHZero(t *testing.T) {
	if got := CollatzSteps(0); got != -1 {
		t.Errorf("CollatzSteps(0) = %d, want -1", got)
	}
}

func TestHReference(t *testing.T) {
	// Independent recomputation for a sweep of inputs, using the same
	// rule but structured differently (recursion) to catch off-by-one.
	var ref func(n uint64) int
	ref = func(n uint64) int {
		if n == 1 {
			return 0
		}
		if n%2 == 0 {
			return 1 + ref(n/2)
		}
		return 1 + ref(3*n+1)
	}
	for n := uint64(1); n <= 5000; n++ {
		want := ref(n)
		if got := CollatzSteps(n); got != want {
			t.Fatalf("CollatzSteps(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestHLargeNoOverflow(t *testing.T) {
	// 97 is known to require 118 steps; larger values still terminate.
	if got := CollatzSteps(97); got != 118 {
		t.Errorf("CollatzSteps(97) = %d, want 118", got)
	}
	// 703 has a long trajectory with large peak (~250504); ensure it
	// completes without overflow of uint64.
	if got := CollatzSteps(703); got != 170 {
		t.Errorf("CollatzSteps(703) = %d, want 170", got)
	}
}
