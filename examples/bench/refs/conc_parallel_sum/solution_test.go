package solution

import "testing"

func seqSum(nums []int) int {
	s := 0
	for _, v := range nums {
		s += v
	}
	return s
}

func TestVBasicSum(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5}
	if got := ParallelSum(nums, 2); got != 15 {
		t.Fatalf("got %d want 15", got)
	}
}

func TestVEmpty(t *testing.T) {
	if got := ParallelSum(nil, 4); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
	if got := ParallelSum([]int{}, 4); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestVNegatives(t *testing.T) {
	nums := []int{-5, 10, -3, 8}
	if got := ParallelSum(nums, 3); got != 10 {
		t.Fatalf("got %d want 10", got)
	}
}

func TestHZeroAndNegativeWorkers(t *testing.T) {
	nums := []int{7, 7, 7, 7}
	for _, w := range []int{0, -1, -50} {
		if got := ParallelSum(nums, w); got != 28 {
			t.Fatalf("workers=%d got %d want 28", w, got)
		}
	}
}

func TestHManyPartitionings(t *testing.T) {
	const n = 10000
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i - 5000
	}
	want := seqSum(nums)
	for _, w := range []int{1, 2, 3, 7, 13, 64, 128, n, n + 100} {
		if got := ParallelSum(nums, w); got != want {
			t.Fatalf("workers=%d got %d want %d", w, got, want)
		}
	}
}

func TestHSingleElement(t *testing.T) {
	if got := ParallelSum([]int{42}, 8); got != 42 {
		t.Fatalf("got %d want 42", got)
	}
}
