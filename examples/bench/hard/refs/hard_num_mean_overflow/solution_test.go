package solution

import (
	"math"
	"testing"
)

func TestVBasic(t *testing.T) {
	cases := []struct {
		in   []int64
		want int64
	}{
		{[]int64{1}, 1},
		{[]int64{1, 2, 3}, 2},
		{[]int64{1, 2, 3, 4}, 2}, // 10/4 = 2.5 -> floor 2
		{[]int64{2, 2, 2}, 2},
		{[]int64{0, 0, 0}, 0},
		{[]int64{10, 20, 30, 40, 50}, 30},
		{[]int64{7, 8}, 7}, // 15/2 = 7.5 -> 7
	}
	for _, c := range cases {
		if got := IntegerMean(c.in); got != c.want {
			t.Errorf("IntegerMean(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestVNegative(t *testing.T) {
	cases := []struct {
		in   []int64
		want int64
	}{
		{[]int64{-1, -2}, -2}, // -1.5 floor -> -2
		{[]int64{-1, -2, -3}, -2},
		{[]int64{-10, 10}, 0},
		{[]int64{-5, 5, -5, 5}, 0},
		{[]int64{-1, 0}, -1},  // -0.5 floor -> -1
		{[]int64{-7, -8}, -8}, // -7.5 floor -> -8
	}
	for _, c := range cases {
		if got := IntegerMean(c.in); got != c.want {
			t.Errorf("IntegerMean(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestHSumOverflowPositive is the trap: the exact sum overflows int64, so a
// naive sum/len implementation produces garbage (or panics on overflow). The
// mathematically correct floored mean of three copies of MaxInt64 is MaxInt64.
func TestHSumOverflowPositive(t *testing.T) {
	max := int64(math.MaxInt64)
	in := []int64{max, max, max}
	// Exact sum = 3*MaxInt64, which overflows int64 badly. Mean = MaxInt64.
	if got := IntegerMean(in); got != max {
		t.Errorf("IntegerMean(3x MaxInt64) = %d, want %d", got, max)
	}
}

// TestHSumOverflowMixed exercises overflow where the true sum is not divisible
// by n and floor rounding matters.
func TestHSumOverflowMixed(t *testing.T) {
	max := int64(math.MaxInt64)
	// sum = MaxInt64 + MaxInt64 + (MaxInt64 - 2) = 3*MaxInt64 - 2.
	// 3*MaxInt64 = 27670116110564327421 (overflows). Divided by 3:
	// (3*MaxInt64 - 2)/3 = MaxInt64 - 1 with remainder 1 -> floor = MaxInt64 - 1.
	in := []int64{max, max, max - 2}
	want := max - 1
	if got := IntegerMean(in); got != want {
		t.Errorf("IntegerMean(%v) = %d, want %d", in, got, want)
	}
}

// TestHSumOverflowNegative exercises overflow toward negative infinity, where
// floor rounding differs from truncation.
func TestHSumOverflowNegative(t *testing.T) {
	min := int64(math.MinInt64)
	in := []int64{min, min, min}
	// Mean = MinInt64 exactly.
	if got := IntegerMean(in); got != min {
		t.Errorf("IntegerMean(3x MinInt64) = %d, want %d", got, min)
	}
}

// TestHSumOverflowNegativeFloor: true sum not divisible, must floor (round
// toward -inf), not truncate toward zero.
func TestHSumOverflowNegativeFloor(t *testing.T) {
	min := int64(math.MinInt64)
	// sum = min + min + (min + 2) = 3*min + 2. Divided by 3 = min + 2/3.
	// Floor of (min + 0.666...) = min. Truncation would also give min here,
	// so use a case that distinguishes: min, min, min+1.
	// sum = 3*min + 1; /3 = min + 1/3; floor = min.
	in := []int64{min, min, min + 1}
	want := min
	if got := IntegerMean(in); got != want {
		t.Errorf("IntegerMean(%v) = %d, want %d", in, got, want)
	}
}

// TestHLargeCountNoOverflow: many large values summing well past MaxInt64.
func TestHLargeCountNoOverflow(t *testing.T) {
	const n = 1000
	v := int64(math.MaxInt64) - 500
	in := make([]int64, n)
	for i := range in {
		in[i] = v
	}
	// All identical, so mean is exactly v regardless of overflow.
	if got := IntegerMean(in); got != v {
		t.Errorf("IntegerMean(1000x %d) = %d, want %d", v, got, v)
	}
}

func TestHFloorDistinguishesTruncation(t *testing.T) {
	// {-3, 1}: sum -2, /2 = -1 exactly. {-3, 2}: sum -1, /2 = -0.5 floor -> -1.
	if got := IntegerMean([]int64{-3, 2}); got != -1 {
		t.Errorf("IntegerMean([-3 2]) = %d, want -1", got)
	}
	// Truncation of -1/2 gives 0, so this catches truncating implementations.
	if got := IntegerMean([]int64{-1, 0, 0, 0}); got != -1 {
		t.Errorf("IntegerMean([-1 0 0 0]) = %d, want -1", got)
	}
}
