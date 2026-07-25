package solution

import (
	"sync/atomic"
	"testing"
)

func TestVSquaresInOrder(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	got := ParallelMap(in, 2, func(x int) int { return x * x })
	want := []int{1, 4, 9, 16, 25}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestVEmptyInput(t *testing.T) {
	got := ParallelMap([]int{}, 4, func(x int) int { return x })
	if got == nil {
		t.Fatal("expected non-nil empty result")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got len %d", len(got))
	}
}

func TestVStringTransform(t *testing.T) {
	in := []string{"a", "b", "c"}
	got := ParallelMap(in, 0, func(s string) string { return s + "!" })
	want := []string{"a!", "b!", "c!"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestHZeroAndNegativeWorkers(t *testing.T) {
	in := []int{10, 20, 30}
	for _, w := range []int{0, -1, -100} {
		got := ParallelMap(in, w, func(x int) int { return x + 1 })
		want := []int{11, 21, 31}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("workers=%d index %d: got %d want %d", w, i, got[i], want[i])
			}
		}
	}
}

func TestHExactlyOnceLargeInput(t *testing.T) {
	const n = 20000
	in := make([]int, n)
	for i := range in {
		in[i] = i
	}
	var calls atomic.Int64
	got := ParallelMap(in, 64, func(x int) int {
		calls.Add(1)
		return x * 2
	})
	if calls.Load() != int64(n) {
		t.Fatalf("f applied %d times, want exactly %d", calls.Load(), n)
	}
	for i := range in {
		if got[i] != i*2 {
			t.Fatalf("index %d: got %d want %d", i, got[i], i*2)
		}
	}
}

func TestHMoreWorkersThanElements(t *testing.T) {
	in := []int{1, 2}
	got := ParallelMap(in, 1000, func(x int) int { return -x })
	if len(got) != 2 || got[0] != -1 || got[1] != -2 {
		t.Fatalf("got %v", got)
	}
}
