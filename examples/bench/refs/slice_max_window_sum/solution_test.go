package solution

import "testing"

// brute recomputes the answer directly for cross-checking.
func brute(xs []int, k int) int {
	if k < 1 || k > len(xs) {
		return 0
	}
	best := 0
	first := true
	for i := 0; i+k <= len(xs); i++ {
		sum := 0
		for j := i; j < i+k; j++ {
			sum += xs[j]
		}
		if first || sum > best {
			best = sum
			first = false
		}
	}
	return best
}

func TestVBasic(t *testing.T) {
	cases := []struct {
		xs   []int
		k    int
		want int
	}{
		{[]int{1, 2, 3, 4, 5}, 2, 9},
		{[]int{1, 2, 3, 4, 5}, 1, 5},
		{[]int{1, 2, 3, 4, 5}, 5, 15},
		{[]int{2, 1, 5, 1, 3, 2}, 3, 9},
		{[]int{5, 4, 3, 2, 1}, 2, 9},
	}
	for _, c := range cases {
		if got := MaxWindowSum(c.xs, c.k); got != c.want {
			t.Errorf("MaxWindowSum(%v, %d) = %d, want %d", c.xs, c.k, got, c.want)
		}
	}
}

func TestVNegatives(t *testing.T) {
	// All negative: the max window is the least-negative run.
	// windows: -3+-1=-4, -1+-4=-5, -4+-2=-6 => max -4.
	if got := MaxWindowSum([]int{-3, -1, -4, -2}, 2); got != -4 {
		t.Errorf("got %d, want -4", got)
	}
	// Mixed signs.
	if got := MaxWindowSum([]int{-2, 5, -1, 3, -4}, 2); got != 4 {
		t.Errorf("got %d, want 4", got)
	}
}

func TestVSingleElement(t *testing.T) {
	if got := MaxWindowSum([]int{42}, 1); got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if got := MaxWindowSum([]int{-7}, 1); got != -7 {
		t.Errorf("got %d, want -7", got)
	}
}

func TestHOutOfRangeK(t *testing.T) {
	if got := MaxWindowSum(nil, 1); got != 0 {
		t.Errorf("nil,1: got %d, want 0", got)
	}
	if got := MaxWindowSum([]int{}, 1); got != 0 {
		t.Errorf("empty,1: got %d, want 0", got)
	}
	if got := MaxWindowSum([]int{1, 2, 3}, 0); got != 0 {
		t.Errorf("k=0: got %d, want 0", got)
	}
	if got := MaxWindowSum([]int{1, 2, 3}, 4); got != 0 {
		t.Errorf("k>len: got %d, want 0", got)
	}
	if got := MaxWindowSum([]int{1, 2, 3}, -1); got != 0 {
		t.Errorf("k<0: got %d, want 0", got)
	}
}

func TestHFullWindowNegative(t *testing.T) {
	// k == len with all negatives; single window is the whole slice.
	if got := MaxWindowSum([]int{-1, -2, -3}, 3); got != -6 {
		t.Errorf("got %d, want -6", got)
	}
}

func TestHAgainstBrute(t *testing.T) {
	// Deterministic pseudo-random cross-check over many shapes.
	var seed uint64 = 12345
	next := func(mod int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		v := int(seed>>33) % mod
		if v < 0 {
			v += mod
		}
		return v
	}
	for trial := 0; trial < 2000; trial++ {
		n := 1 + next(30)
		xs := make([]int, n)
		for i := range xs {
			xs[i] = next(201) - 100 // -100..100
		}
		k := 1 + next(n)
		want := brute(xs, k)
		if got := MaxWindowSum(xs, k); got != want {
			t.Fatalf("MaxWindowSum(%v, %d) = %d, want %d", xs, k, got, want)
		}
	}
}

func TestHLargeInput(t *testing.T) {
	n := 100000
	xs := make([]int, n)
	for i := range xs {
		xs[i] = 1
	}
	xs[n-1] = 1000 // last element large; best window includes it
	if got := MaxWindowSum(xs, 3); got != 1002 {
		t.Errorf("got %d, want 1002", got)
	}
}
