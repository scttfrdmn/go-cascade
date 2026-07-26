package solution

import "testing"

func TestVMixed(t *testing.T) {
	// [4,-1,2,1] sums to 6.
	nums := []int{-2, 1, -3, 4, -1, 2, 1, -5, 4}
	if got := MaxSubarraySum(nums); got != 6 {
		t.Errorf("MaxSubarraySum = %d, want 6", got)
	}
}

func TestVAllPositive(t *testing.T) {
	if got := MaxSubarraySum([]int{1, 2, 3, 4}); got != 10 {
		t.Errorf("MaxSubarraySum = %d, want 10", got)
	}
}

func TestHAllNegative(t *testing.T) {
	if got := MaxSubarraySum([]int{-3, -1, -4, -2}); got != -1 {
		t.Errorf("MaxSubarraySum = %d, want -1", got)
	}
}

func TestHSingle(t *testing.T) {
	if got := MaxSubarraySum([]int{-7}); got != -7 {
		t.Errorf("MaxSubarraySum([-7]) = %d, want -7", got)
	}
	if got := MaxSubarraySum([]int{42}); got != 42 {
		t.Errorf("MaxSubarraySum([42]) = %d, want 42", got)
	}
}

func TestHEmpty(t *testing.T) {
	if got := MaxSubarraySum(nil); got != 0 {
		t.Errorf("MaxSubarraySum(nil) = %d, want 0", got)
	}
	if got := MaxSubarraySum([]int{}); got != 0 {
		t.Errorf("MaxSubarraySum([]) = %d, want 0", got)
	}
}

func TestHLarge(t *testing.T) {
	n := 100000
	nums := make([]int, n)
	for i := range nums {
		nums[i] = -1
	}
	nums[n/2] = 1000000
	if got := MaxSubarraySum(nums); got != 1000000 {
		t.Errorf("MaxSubarraySum large = %d, want 1000000", got)
	}
}

func TestHZeroWithNegatives(t *testing.T) {
	if got := MaxSubarraySum([]int{-1, 0, -2}); got != 0 {
		t.Errorf("MaxSubarraySum = %d, want 0", got)
	}
}
