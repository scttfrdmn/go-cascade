package solution

import (
	"reflect"
	"testing"
)

func TestVBasic(t *testing.T) {
	got := MoveZeros([]int{0, 1, 0, 3, 12})
	if !reflect.DeepEqual(got, []int{1, 3, 12, 0, 0}) {
		t.Fatalf("got %v", got)
	}
}

func TestVOrder(t *testing.T) {
	got := MoveZeros([]int{4, 0, 5, 0, 6})
	if !reflect.DeepEqual(got, []int{4, 5, 6, 0, 0}) {
		t.Fatalf("got %v", got)
	}
}

func TestHEmptyAndNil(t *testing.T) {
	got := MoveZeros(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("nil: %v", got)
	}
	got = MoveZeros([]int{})
	if got == nil || len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
}

func TestHAllZeros(t *testing.T) {
	got := MoveZeros([]int{0, 0, 0})
	if !reflect.DeepEqual(got, []int{0, 0, 0}) {
		t.Fatalf("got %v", got)
	}
}

func TestHNoZeros(t *testing.T) {
	got := MoveZeros([]int{1, 2, 3})
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("got %v", got)
	}
}

func TestHNegativesPreserved(t *testing.T) {
	got := MoveZeros([]int{0, -1, 0, -2})
	if !reflect.DeepEqual(got, []int{-1, -2, 0, 0}) {
		t.Fatalf("got %v", got)
	}
}

func TestHNoMutation(t *testing.T) {
	in := []int{0, 1, 0, 3}
	cp := append([]int(nil), in...)
	got := MoveZeros(in)
	if !reflect.DeepEqual(in, cp) {
		t.Fatalf("input mutated: %v", in)
	}
	// Result must be a distinct backing array.
	if len(got) > 0 && len(in) > 0 {
		got[0] = 999
		if in[0] == 999 {
			t.Fatal("result shares backing array with input")
		}
	}
}
