package solution

import "testing"

func TestVBasic(t *testing.T) {
	cases := []struct {
		in   []int
		want int
	}{
		{[]int{12}, 12},
		{[]int{12, 18}, 6},
		{[]int{12, 18, 24}, 6},
		{[]int{7, 13}, 1},
		{[]int{100, 10, 5}, 5},
		{[]int{9, 27, 81}, 9},
		{[]int{2, 3, 5, 7}, 1},
	}
	for _, c := range cases {
		if got := GCDSlice(c.in); got != c.want {
			t.Fatalf("GCDSlice(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestVSingle(t *testing.T) {
	if got := GCDSlice([]int{42}); got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestHAllEqual(t *testing.T) {
	if got := GCDSlice([]int{8, 8, 8, 8}); got != 8 {
		t.Fatalf("got %d, want 8", got)
	}
}

func TestHOnePresentForcesOne(t *testing.T) {
	if got := GCDSlice([]int{1, 999983, 12}); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
}

func TestHLargeCoprime(t *testing.T) {
	// two large coprime primes
	if got := GCDSlice([]int{999983, 999979}); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
}

func TestHOrderIndependent(t *testing.T) {
	a := GCDSlice([]int{18, 24, 12})
	b := GCDSlice([]int{24, 12, 18})
	c := GCDSlice([]int{12, 18, 24})
	if a != 6 || b != 6 || c != 6 {
		t.Fatalf("order dependent or wrong: %d %d %d", a, b, c)
	}
}

func TestHEmptyPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty slice")
		}
	}()
	GCDSlice(nil)
}

func TestHLargeSlice(t *testing.T) {
	xs := make([]int, 10000)
	for i := range xs {
		xs[i] = 6 * (i + 1)
	}
	if got := GCDSlice(xs); got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
}
