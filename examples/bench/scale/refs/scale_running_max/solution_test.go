package solution

import "testing"

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestVIncreasing(t *testing.T) {
	got := RunningMax([]int{1, 2, 3, 4})
	want := []int{1, 2, 3, 4}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVMixed(t *testing.T) {
	got := RunningMax([]int{3, 1, 4, 1, 5, 9, 2})
	want := []int{3, 3, 4, 4, 5, 9, 9}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVDecreasing(t *testing.T) {
	got := RunningMax([]int{5, 4, 3, 2, 1})
	want := []int{5, 5, 5, 5, 5}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHEmptyNonNil(t *testing.T) {
	got := RunningMax([]int{})
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHNilNonNil(t *testing.T) {
	got := RunningMax(nil)
	if got == nil {
		t.Fatal("expected non-nil for nil input")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHSingle(t *testing.T) {
	got := RunningMax([]int{42})
	want := []int{42}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHNegatives(t *testing.T) {
	got := RunningMax([]int{-5, -3, -8, -1})
	want := []int{-5, -3, -3, -1}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHInputNotModified(t *testing.T) {
	in := []int{3, 1, 4, 1, 5}
	orig := []int{3, 1, 4, 1, 5}
	_ = RunningMax(in)
	if !equalInts(in, orig) {
		t.Fatalf("input mutated: %v", in)
	}
}
