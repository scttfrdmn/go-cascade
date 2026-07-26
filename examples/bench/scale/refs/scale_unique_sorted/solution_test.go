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

func TestVBasic(t *testing.T) {
	got := UniqueSorted([]int{3, 1, 2, 3, 1})
	want := []int{1, 2, 3}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVAlreadyUnique(t *testing.T) {
	got := UniqueSorted([]int{5, 4, 3})
	want := []int{3, 4, 5}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVNegatives(t *testing.T) {
	got := UniqueSorted([]int{-1, -3, -1, 0, 2, 2})
	want := []int{-3, -1, 0, 2}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHEmptyNonNil(t *testing.T) {
	got := UniqueSorted([]int{})
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHNilNonNil(t *testing.T) {
	got := UniqueSorted(nil)
	if got == nil {
		t.Fatal("expected non-nil for nil input")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHAllSame(t *testing.T) {
	got := UniqueSorted([]int{7, 7, 7, 7})
	want := []int{7}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHInputNotModified(t *testing.T) {
	in := []int{3, 1, 2, 1}
	orig := []int{3, 1, 2, 1}
	_ = UniqueSorted(in)
	if !equalInts(in, orig) {
		t.Fatalf("input mutated: %v", in)
	}
}
