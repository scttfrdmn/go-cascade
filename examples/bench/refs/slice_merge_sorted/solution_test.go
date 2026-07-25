package solution

import (
	"reflect"
	"sort"
	"testing"
)

func TestVMergeBasic(t *testing.T) {
	got := MergeSorted([]int{1, 3, 5}, []int{2, 4, 6})
	if !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6}) {
		t.Errorf("got %v", got)
	}
}

func TestVMergeDuplicates(t *testing.T) {
	got := MergeSorted([]int{1, 2, 2}, []int{2, 3})
	if !reflect.DeepEqual(got, []int{1, 2, 2, 2, 3}) {
		t.Errorf("got %v", got)
	}
}

func TestVMergeOneEmpty(t *testing.T) {
	if got := MergeSorted(nil, []int{1, 2}); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("got %v, want [1 2]", got)
	}
	if got := MergeSorted([]int{1, 2}, nil); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("got %v, want [1 2]", got)
	}
}

func TestHMergeBothEmpty(t *testing.T) {
	got := MergeSorted(nil, nil)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestHMergeDisjointRanges(t *testing.T) {
	got := MergeSorted([]int{4, 5, 6}, []int{1, 2, 3})
	if !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6}) {
		t.Errorf("got %v", got)
	}
}

func TestHMergeNoMutation(t *testing.T) {
	a := []int{1, 3}
	b := []int{2, 4}
	oa := append([]int(nil), a...)
	ob := append([]int(nil), b...)
	_ = MergeSorted(a, b)
	if !reflect.DeepEqual(a, oa) || !reflect.DeepEqual(b, ob) {
		t.Errorf("inputs mutated: a=%v b=%v", a, b)
	}
}

func TestHMergeAgainstSort(t *testing.T) {
	a := []int{-5, -5, 0, 3, 3, 100}
	b := []int{-10, -5, 1, 3, 50, 50, 200}
	got := MergeSorted(a, b)
	combined := append(append([]int(nil), a...), b...)
	sort.Ints(combined)
	if !reflect.DeepEqual(got, combined) {
		t.Errorf("got %v, want %v", got, combined)
	}
}

func TestHMergeNegatives(t *testing.T) {
	got := MergeSorted([]int{-3, -1}, []int{-2, 0})
	if !reflect.DeepEqual(got, []int{-3, -2, -1, 0}) {
		t.Errorf("got %v", got)
	}
}
