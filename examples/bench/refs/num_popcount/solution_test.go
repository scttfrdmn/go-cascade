package solution

import (
	"math/bits"
	"testing"
)

func TestVBasic(t *testing.T) {
	cases := []struct {
		in   uint64
		want int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 2},
		{4, 1},
		{7, 3},
		{8, 1},
		{255, 8},
		{256, 1},
		{1023, 10},
	}
	for _, c := range cases {
		if got := PopCount(c.in); got != c.want {
			t.Errorf("PopCount(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestVPowersOfTwo(t *testing.T) {
	for i := 0; i < 64; i++ {
		var n uint64 = 1 << i
		if got := PopCount(n); got != 1 {
			t.Errorf("PopCount(1<<%d) = %d, want 1", i, got)
		}
	}
}

func TestHMaxUint64(t *testing.T) {
	const max = ^uint64(0)
	if got := PopCount(max); got != 64 {
		t.Errorf("PopCount(max) = %d, want 64", got)
	}
}

func TestHAgainstStdlib(t *testing.T) {
	// Deterministic pseudo-random sweep compared to bits.OnesCount64.
	var x uint64 = 0x9E3779B97F4A7C15
	for i := 0; i < 10000; i++ {
		x = x*6364136223846793005 + 1442695040888963407
		want := bits.OnesCount64(x)
		if got := PopCount(x); got != want {
			t.Fatalf("PopCount(%d) = %d, want %d", x, got, want)
		}
	}
}

func TestHLowHighBits(t *testing.T) {
	// Only the highest bit set.
	if got := PopCount(1 << 63); got != 1 {
		t.Errorf("PopCount(1<<63) = %d, want 1", got)
	}
	// Alternating bits 0b...0101 across 64 bits => 32 ones.
	var alt uint64 = 0x5555555555555555
	if got := PopCount(alt); got != 32 {
		t.Errorf("PopCount(alt) = %d, want 32", got)
	}
	// Other alternation => 32 ones.
	var alt2 uint64 = 0xAAAAAAAAAAAAAAAA
	if got := PopCount(alt2); got != 32 {
		t.Errorf("PopCount(alt2) = %d, want 32", got)
	}
}
