package solution

import (
	"math"
	"testing"
)

func TestVReverseInt(t *testing.T) {
	cases := []struct {
		in   int32
		want int32
	}{
		{123, 321},
		{-120, -21},
		{0, 0},
		{5, 5},
		{-8, -8},
		{100, 1},
		{-1000, -1},
	}
	for _, c := range cases {
		if got := ReverseInt(c.in); got != c.want {
			t.Errorf("ReverseInt(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestHReverseInt(t *testing.T) {
	// Overflow on reversal: 1534236469 reversed is 9646324351 > MaxInt32.
	if got := ReverseInt(1534236469); got != 0 {
		t.Errorf("ReverseInt(1534236469) = %d, want 0 (overflow)", got)
	}
	// Negative overflow.
	if got := ReverseInt(-1534236469); got != 0 {
		t.Errorf("ReverseInt(-1534236469) = %d, want 0 (overflow)", got)
	}
	// MaxInt32 = 2147483647 -> 7463847412 overflows.
	if got := ReverseInt(math.MaxInt32); got != 0 {
		t.Errorf("ReverseInt(MaxInt32) = %d, want 0 (overflow)", got)
	}
	// MinInt32 = -2147483648 -> -8463847412 overflows.
	if got := ReverseInt(math.MinInt32); got != 0 {
		t.Errorf("ReverseInt(MinInt32) = %d, want 0 (overflow)", got)
	}
	// A value whose reversal exactly fits: 1463847412 -> 2147483641 <= MaxInt32.
	if got := ReverseInt(1463847412); got != 2147483641 {
		t.Errorf("ReverseInt(1463847412) = %d, want 2147483641", got)
	}
	// Trailing zeros collapse and sign is preserved.
	if got := ReverseInt(-2100000000); got != -12 {
		t.Errorf("ReverseInt(-2100000000) = %d, want -12", got)
	}
}
