package solution

import (
	"math"
	"testing"
)

func TestVSumDigits(t *testing.T) {
	cases := []struct {
		in   uint64
		want int
	}{
		{0, 0},
		{5, 5},
		{9, 9},
		{10, 1},
		{123, 6},
		{999, 27},
		{100000, 1},
	}
	for _, c := range cases {
		if got := SumDigits(c.in); got != c.want {
			t.Errorf("SumDigits(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestHSumDigits(t *testing.T) {
	// Boundary: single digit boundaries.
	if got := SumDigits(0); got != 0 {
		t.Errorf("SumDigits(0) = %d, want 0", got)
	}
	// Max uint64 = 18446744073709551615, digit sum = 87.
	if got := SumDigits(math.MaxUint64); got != 87 {
		t.Errorf("SumDigits(MaxUint64) = %d, want 87", got)
	}
	// Repunit-like: many nines.
	if got := SumDigits(9999999999); got != 90 {
		t.Errorf("SumDigits(9999999999) = %d, want 90", got)
	}
	// Trailing zeros contribute nothing.
	if got := SumDigits(1000000000000); got != 1 {
		t.Errorf("SumDigits(1e12) = %d, want 1", got)
	}
}
