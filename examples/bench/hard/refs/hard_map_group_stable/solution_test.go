package solution

import (
	"reflect"
	"testing"
)

func TestVGroupBasic(t *testing.T) {
	in := []Pair{
		{"a", 1},
		{"b", 2},
		{"a", 3},
	}
	got := GroupStable(in)
	want := map[string][]int{
		"a": {1, 3},
		"b": {2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupStable(%v) = %v, want %v", in, got, want)
	}
}

func TestVGroupEmpty(t *testing.T) {
	got := GroupStable(nil)
	if got == nil {
		t.Fatal("GroupStable(nil) returned nil map, want empty non-nil map")
	}
	if len(got) != 0 {
		t.Errorf("GroupStable(nil) = %v, want empty", got)
	}
}

// TestHGroupOrderPreserved is THE TRAP: values per key must preserve first-seen
// input order exactly, even when keys are interleaved and values are not sorted.
func TestHGroupOrderPreserved(t *testing.T) {
	in := []Pair{
		{"x", 5},
		{"y", 9},
		{"x", 1},
		{"z", 7},
		{"y", 2},
		{"x", 8},
		{"y", 4},
	}
	got := GroupStable(in)
	want := map[string][]int{
		"x": {5, 1, 8},
		"y": {9, 2, 4},
		"z": {7},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GroupStable(%v) = %v, want %v", in, got, want)
	}
	// Explicitly assert ordering (not just multiset equality) for key "x".
	if got["x"][0] != 5 || got["x"][1] != 1 || got["x"][2] != 8 {
		t.Errorf("key x values %v not in first-seen order [5 1 8]", got["x"])
	}
}

// TestHGroupDuplicatesAndReverse guards against implementations that sort or
// reverse values within a key.
func TestHGroupDuplicatesAndReverse(t *testing.T) {
	in := []Pair{
		{"k", 3},
		{"k", 3},
		{"k", 1},
		{"k", 2},
	}
	got := GroupStable(in)
	want := []int{3, 3, 1, 2} // insertion order, duplicates kept, not sorted
	if !reflect.DeepEqual(got["k"], want) {
		t.Errorf("GroupStable value order = %v, want %v", got["k"], want)
	}
}
