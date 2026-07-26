package solution

import "testing"

func equal2D(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func TestVEvenSplit(t *testing.T) {
	got := Chunk([]int{1, 2, 3, 4}, 2)
	want := [][]int{{1, 2}, {3, 4}}
	if !equal2D(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVShorterFinalChunk(t *testing.T) {
	got := Chunk([]int{1, 2, 3, 4, 5}, 2)
	want := [][]int{{1, 2}, {3, 4}, {5}}
	if !equal2D(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVSizeOne(t *testing.T) {
	got := Chunk([]int{7, 8, 9}, 1)
	want := [][]int{{7}, {8}, {9}}
	if !equal2D(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVSizeLargerThanInput(t *testing.T) {
	got := Chunk([]int{1, 2}, 10)
	want := [][]int{{1, 2}}
	if !equal2D(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHEmptyNonNil(t *testing.T) {
	got := Chunk([]int{}, 3)
	if got == nil {
		t.Fatal("expected non-nil empty result")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHNilInputNonNil(t *testing.T) {
	got := Chunk(nil, 3)
	if got == nil {
		t.Fatal("expected non-nil for nil input")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHInputNotModified(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	orig := []int{1, 2, 3, 4, 5}
	out := Chunk(in, 2)
	for i := range in {
		if in[i] != orig[i] {
			t.Fatalf("input mutated at %d: %v", i, in)
		}
	}
	// Mutating the output must not affect the input.
	out[0][0] = 999
	if in[0] != 1 {
		t.Fatalf("output shares backing array with input")
	}
}

func TestHBoundaryExactMultiple(t *testing.T) {
	got := Chunk([]int{1, 2, 3, 4, 5, 6}, 3)
	want := [][]int{{1, 2, 3}, {4, 5, 6}}
	if !equal2D(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
