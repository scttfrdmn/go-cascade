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
	got := Flatten([][]int{{1, 2}, {3}, {4, 5, 6}})
	want := []int{1, 2, 3, 4, 5, 6}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVOrderPreserved(t *testing.T) {
	got := Flatten([][]int{{3}, {1}, {2}})
	want := []int{3, 1, 2}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHEmptyOuterNonNil(t *testing.T) {
	got := Flatten([][]int{})
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHNilOuterNonNil(t *testing.T) {
	got := Flatten(nil)
	if got == nil {
		t.Fatal("expected non-nil empty slice for nil input")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHNilAndEmptyInner(t *testing.T) {
	got := Flatten([][]int{nil, {}, {7}, nil, {}, {8, 9}})
	want := []int{7, 8, 9}
	if !equalInts(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHAllEmptyInner(t *testing.T) {
	got := Flatten([][]int{nil, {}, nil})
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHInputNotModified(t *testing.T) {
	inner := []int{1, 2}
	nested := [][]int{inner, {3}}
	_ = Flatten(nested)
	if !equalInts(inner, []int{1, 2}) {
		t.Fatalf("inner slice mutated: %v", inner)
	}
	if len(nested) != 2 {
		t.Fatalf("outer slice mutated")
	}
}
