package solution

import (
	"reflect"
	"testing"
)

func TestVBasic(t *testing.T) {
	got := Intersection([]int{1, 2, 3, 4}, []int{2, 4, 6})
	if !reflect.DeepEqual(got, []int{2, 4}) {
		t.Fatalf("got %v", got)
	}
}

func TestVSorted(t *testing.T) {
	got := Intersection([]int{5, 3, 1}, []int{1, 3, 5})
	if !reflect.DeepEqual(got, []int{1, 3, 5}) {
		t.Fatalf("got %v", got)
	}
}

func TestHNoOverlap(t *testing.T) {
	got := Intersection([]int{1, 2}, []int{3, 4})
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestHEmptyInputs(t *testing.T) {
	got := Intersection(nil, nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("nil inputs: %v", got)
	}
	got = Intersection([]int{}, []int{1})
	if got == nil || len(got) != 0 {
		t.Fatalf("empty a: %v", got)
	}
}

func TestHDuplicates(t *testing.T) {
	got := Intersection([]int{1, 1, 2, 2, 2}, []int{2, 2, 1, 1})
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("distinct only: %v", got)
	}
}

func TestHNegatives(t *testing.T) {
	got := Intersection([]int{-3, 0, 5}, []int{5, -3, -3})
	if !reflect.DeepEqual(got, []int{-3, 5}) {
		t.Fatalf("got %v", got)
	}
}

func TestHNoMutation(t *testing.T) {
	a := []int{3, 1, 2}
	b := []int{2, 1, 4}
	ac := append([]int(nil), a...)
	bc := append([]int(nil), b...)
	Intersection(a, b)
	if !reflect.DeepEqual(a, ac) || !reflect.DeepEqual(b, bc) {
		t.Fatalf("inputs mutated: a=%v b=%v", a, b)
	}
}
