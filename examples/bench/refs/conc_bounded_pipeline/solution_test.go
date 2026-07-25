package solution

import (
	"strconv"
	"sync/atomic"
	"testing"
)

func TestVTwoStagesInOrder(t *testing.T) {
	in := []int{1, 2, 3, 4}
	// stage1: x -> x+1 ; stage2: y -> y*10
	got := Pipeline(in, func(x int) int { return x + 1 }, func(y int) int { return y * 10 })
	want := []int{20, 30, 40, 50}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestVTypeChange(t *testing.T) {
	in := []int{1, 2, 3}
	got := Pipeline(in, func(x int) string { return strconv.Itoa(x) }, func(s string) string { return s + "!" })
	want := []string{"1!", "2!", "3!"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestVEmpty(t *testing.T) {
	got := Pipeline([]int{}, func(x int) int { return x }, func(y int) int { return y })
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHLargeOrderAndExactlyOnce(t *testing.T) {
	const n = 20000
	in := make([]int, n)
	for i := range in {
		in[i] = i
	}
	var s1, s2 atomic.Int64
	got := Pipeline(in,
		func(x int) int { s1.Add(1); return x * 2 },
		func(y int) int { s2.Add(1); return y + 1 },
	)
	if s1.Load() != n || s2.Load() != n {
		t.Fatalf("stage calls s1=%d s2=%d want %d each", s1.Load(), s2.Load(), n)
	}
	for i := range in {
		want := i*2 + 1
		if got[i] != want {
			t.Fatalf("index %d: got %d want %d", i, got[i], want)
		}
	}
}

func TestHSingleElement(t *testing.T) {
	got := Pipeline([]int{9}, func(x int) int { return x - 1 }, func(y int) int { return y * y })
	if len(got) != 1 || got[0] != 64 {
		t.Fatalf("got %v want [64]", got)
	}
}
