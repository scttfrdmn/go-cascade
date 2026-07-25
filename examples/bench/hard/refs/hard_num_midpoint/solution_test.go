package solution

import (
	"math"
	"testing"
)

func TestVBasic(t *testing.T) {
	cases := []struct {
		low, high, want int
	}{
		{0, 0, 0},
		{0, 1, 0}, // 0.5 -> 0
		{0, 2, 1},
		{2, 2, 2},
		{1, 4, 2}, // 5/2 -> 2
		{3, 7, 5}, // 10/2 -> 5
		{10, 20, 15},
		{5, 6, 5}, // 11/2 -> 5
	}
	for _, c := range cases {
		if got := Midpoint(c.low, c.high); got != c.want {
			t.Errorf("Midpoint(%d, %d) = %d, want %d", c.low, c.high, got, c.want)
		}
	}
}

// TestHMaxIntOverflow is the trap: low and high are both near MaxInt, so the
// naive (low+high)/2 overflows to a negative value. The correct midpoint is a
// large positive number.
func TestHMaxIntOverflow(t *testing.T) {
	max := math.MaxInt
	// Both equal to MaxInt: midpoint is MaxInt. Naive sum overflows to -2.
	if got := Midpoint(max, max); got != max {
		t.Errorf("Midpoint(MaxInt, MaxInt) = %d, want %d", got, max)
	}
}

func TestHMaxIntOverflowFloor(t *testing.T) {
	max := math.MaxInt
	// low=max-1, high=max. True midpoint = max-1 + 1/2 -> floor max-1.
	// Naive sum (2*max-1) overflows.
	want := max - 1
	if got := Midpoint(max-1, max); got != want {
		t.Errorf("Midpoint(MaxInt-1, MaxInt) = %d, want %d", got, want)
	}
}

func TestHNearMaxSpread(t *testing.T) {
	max := math.MaxInt
	// low=0, high=max: midpoint = max/2. No overflow here, sanity check.
	if got := Midpoint(0, max); got != max/2 {
		t.Errorf("Midpoint(0, MaxInt) = %d, want %d", got, max/2)
	}
	// low=1, high=max: sum overflows in naive form. Midpoint = 1 + (max-1)/2.
	want := 1 + (max-1)/2
	if got := Midpoint(1, max); got != want {
		t.Errorf("Midpoint(1, MaxInt) = %d, want %d", got, want)
	}
}

func TestHBothLargeEven(t *testing.T) {
	max := math.MaxInt
	// Two large values whose sum overflows; difference small and even.
	low := max - 10
	high := max - 2
	want := low + (high-low)/2 // = max - 6
	if got := Midpoint(low, high); got != want {
		t.Errorf("Midpoint(%d, %d) = %d, want %d", low, high, got, want)
	}
	if want != max-6 {
		t.Fatalf("test arithmetic wrong: want=%d, max-6=%d", want, max-6)
	}
}
