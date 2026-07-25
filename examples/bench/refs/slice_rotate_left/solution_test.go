package solution

import (
	"reflect"
	"testing"
)

func TestVRotateLeftBasic(t *testing.T) {
	got := RotateLeft([]int{1, 2, 3, 4, 5}, 2)
	want := []int{3, 4, 5, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestVRotateLeftZero(t *testing.T) {
	got := RotateLeft([]int{1, 2, 3}, 0)
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("got %v, want [1 2 3]", got)
	}
}

func TestVRotateLeftExceedsLength(t *testing.T) {
	got := RotateLeft([]int{1, 2, 3}, 4) // 4 mod 3 = 1
	if !reflect.DeepEqual(got, []int{2, 3, 1}) {
		t.Errorf("got %v, want [2 3 1]", got)
	}
}

func TestVRotateLeftFullCycle(t *testing.T) {
	got := RotateLeft([]int{1, 2, 3, 4}, 4)
	if !reflect.DeepEqual(got, []int{1, 2, 3, 4}) {
		t.Errorf("got %v, want [1 2 3 4]", got)
	}
}

func TestHRotateLeftDoesNotMutate(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	orig := append([]int(nil), in...)
	_ = RotateLeft(in, 3)
	if !reflect.DeepEqual(in, orig) {
		t.Errorf("input mutated: got %v, want %v", in, orig)
	}
}

func TestHRotateLeftNewBacking(t *testing.T) {
	in := []int{9, 8, 7}
	out := RotateLeft(in, 1)
	out[0] = -1
	if in[1] == -1 {
		t.Error("output shares backing array with input")
	}
}

func TestHRotateLeftEmpty(t *testing.T) {
	got := RotateLeft(nil, 3)
	if got == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("got len %d, want 0", len(got))
	}
	if got2 := RotateLeft([]int{}, 0); len(got2) != 0 {
		t.Errorf("got len %d, want 0", len(got2))
	}
}

func TestHRotateLeftSingle(t *testing.T) {
	in := []int{42}
	for _, k := range []int{0, 1, 2, 100} {
		if got := RotateLeft(in, k); !reflect.DeepEqual(got, []int{42}) {
			t.Errorf("k=%d: got %v, want [42]", k, got)
		}
	}
}
