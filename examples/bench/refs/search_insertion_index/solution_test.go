package solution

import "testing"

func TestVSearchInsertionPresent(t *testing.T) {
	nums := []int{1, 3, 5, 6}
	if got := SearchInsertionIndex(nums, 5); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

func TestVSearchInsertionAbsent(t *testing.T) {
	nums := []int{1, 3, 5, 6}
	cases := map[int]int{2: 1, 7: 4, 0: 0}
	for target, want := range cases {
		if got := SearchInsertionIndex(nums, target); got != want {
			t.Errorf("SearchInsertionIndex(%v, %d) = %d, want %d", nums, target, got, want)
		}
	}
}

func TestVSearchInsertionLeftmostDuplicate(t *testing.T) {
	nums := []int{1, 2, 2, 2, 3}
	if got := SearchInsertionIndex(nums, 2); got != 1 {
		t.Errorf("got %d, want 1 (leftmost)", got)
	}
}

func TestHSearchInsertionEmpty(t *testing.T) {
	if got := SearchInsertionIndex(nil, 5); got != 0 {
		t.Errorf("nil slice: got %d, want 0", got)
	}
	if got := SearchInsertionIndex([]int{}, 5); got != 0 {
		t.Errorf("empty slice: got %d, want 0", got)
	}
}

func TestHSearchInsertionBoundaries(t *testing.T) {
	nums := []int{-10, -3, 0, 4, 9}
	if got := SearchInsertionIndex(nums, -100); got != 0 {
		t.Errorf("below all: got %d, want 0", got)
	}
	if got := SearchInsertionIndex(nums, 100); got != len(nums) {
		t.Errorf("above all: got %d, want %d", got, len(nums))
	}
}

func TestHSearchInsertionBruteForce(t *testing.T) {
	nums := []int{-5, -5, 0, 0, 0, 3, 8, 8, 12}
	for target := -8; target <= 15; target++ {
		want := 0
		for _, v := range nums {
			if v < target {
				want++
			}
		}
		if got := SearchInsertionIndex(nums, target); got != want {
			t.Errorf("target %d: got %d, want %d", target, got, want)
		}
	}
}
