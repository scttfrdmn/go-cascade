package solution

import (
	"sync/atomic"
	"testing"
)

func TestVEvensInOrder(t *testing.T) {
	in := []int{1, 2, 3, 4, 5, 6}
	got := ParallelFilter(in, 3, func(x int) bool { return x%2 == 0 })
	want := []int{2, 4, 6}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestVEmpty(t *testing.T) {
	got := ParallelFilter([]int{}, 4, func(x int) bool { return true })
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestVKeepNone(t *testing.T) {
	in := []int{1, 3, 5}
	got := ParallelFilter(in, 0, func(x int) bool { return false })
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHZeroAndNegativeWorkers(t *testing.T) {
	in := []int{10, 15, 20, 25, 30}
	for _, w := range []int{0, -1, -20} {
		got := ParallelFilter(in, w, func(x int) bool { return x > 15 })
		want := []int{20, 25, 30}
		if len(got) != len(want) {
			t.Fatalf("workers=%d got %v want %v", w, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("workers=%d index %d: got %d want %d", w, i, got[i], want[i])
			}
		}
	}
}

func TestHExactlyOnceLargeOrderPreserved(t *testing.T) {
	const n = 20000
	in := make([]int, n)
	for i := range in {
		in[i] = i
	}
	var calls atomic.Int64
	got := ParallelFilter(in, 64, func(x int) bool {
		calls.Add(1)
		return x%3 == 0
	})
	if calls.Load() != int64(n) {
		t.Fatalf("keep called %d times want %d", calls.Load(), n)
	}
	prev := -1
	for _, v := range got {
		if v%3 != 0 {
			t.Fatalf("unexpected element %d", v)
		}
		if v <= prev {
			t.Fatalf("order violated: %d after %d", v, prev)
		}
		prev = v
	}
	if len(got) != (n+2)/3 {
		t.Fatalf("kept %d elements want %d", len(got), (n+2)/3)
	}
}
