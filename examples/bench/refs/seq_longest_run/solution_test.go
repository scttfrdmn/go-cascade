package solution

import "testing"

func TestVBasic(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want int
	}{
		{"empty", []int{}, 0},
		{"single", []int{5}, 1},
		{"all increasing", []int{1, 2, 3, 4}, 4},
		{"all decreasing", []int{4, 3, 2, 1}, 1},
		{"run in middle", []int{5, 1, 2, 3, 1}, 3},
		{"plateau breaks run", []int{1, 2, 2, 3}, 2},
		{"reset then longer run", []int{1, 2, 0, 3, 4}, 3},
		{"early run longest", []int{1, 2, 3, 0, 4}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LongestIncreasingRun(c.in); got != c.want {
				t.Fatalf("LongestIncreasingRun(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestVTrailingRun(t *testing.T) {
	if got := LongestIncreasingRun([]int{5, 4, 1, 2, 3, 4}); got != 4 {
		t.Fatalf("got %d, want 4", got)
	}
}

func TestHNil(t *testing.T) {
	if got := LongestIncreasingRun(nil); got != 0 {
		t.Fatalf("nil: got %d, want 0", got)
	}
}

func TestHEqualElements(t *testing.T) {
	if got := LongestIncreasingRun([]int{7, 7, 7, 7}); got != 1 {
		t.Fatalf("equal elements: got %d, want 1", got)
	}
}

func TestHNegativesAndBoundaries(t *testing.T) {
	if got := LongestIncreasingRun([]int{-3, -2, -1, 0}); got != 4 {
		t.Fatalf("negatives: got %d, want 4", got)
	}
	// strictly increasing requires >, not >=
	if got := LongestIncreasingRun([]int{-1, -1, 0}); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}

func TestHLarge(t *testing.T) {
	n := 100000
	xs := make([]int, n)
	for i := range xs {
		xs[i] = i
	}
	// break the run at the very end so best run is n-1 from start... actually
	// full increasing sequence is length n.
	if got := LongestIncreasingRun(xs); got != n {
		t.Fatalf("large increasing: got %d, want %d", got, n)
	}
	// now insert a drop in the middle
	xs[n/2] = -1
	// left part length n/2, right part length n - n/2 - 1... left is longer or equal
	left := n / 2
	right := n - n/2 - 1
	want := left
	if right > want {
		want = right
	}
	if got := LongestIncreasingRun(xs); got != want {
		t.Fatalf("large with drop: got %d, want %d", got, want)
	}
}
