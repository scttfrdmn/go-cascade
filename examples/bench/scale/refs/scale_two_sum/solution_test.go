package solution

import "testing"

func TestVBasic(t *testing.T) {
	if got := TwoSum([]int{2, 7, 11, 15}, 9); got != [2]int{0, 1} {
		t.Errorf("TwoSum = %v, want [0 1]", got)
	}
}

func TestVMiddle(t *testing.T) {
	if got := TwoSum([]int{3, 2, 4}, 6); got != [2]int{1, 2} {
		t.Errorf("TwoSum = %v, want [1 2]", got)
	}
}

func TestHNegatives(t *testing.T) {
	if got := TwoSum([]int{-3, 4, 3, 90}, 0); got != [2]int{0, 2} {
		t.Errorf("TwoSum = %v, want [0 2]", got)
	}
}

func TestHDuplicateValues(t *testing.T) {
	// Two equal elements summing to target; smallest index first.
	if got := TwoSum([]int{3, 3}, 6); got != [2]int{0, 1} {
		t.Errorf("TwoSum([3,3],6) = %v, want [0 1]", got)
	}
}

func TestHEndsPair(t *testing.T) {
	if got := TwoSum([]int{1, 5, 2, 8, 4}, 12); got != [2]int{3, 4} {
		t.Errorf("TwoSum = %v, want [3 4]", got)
	}
}

func TestHLarge(t *testing.T) {
	n := 50000
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i
	}
	// The only pair summing to 2n-3 is (n-2) + (n-1).
	got := TwoSum(nums, 2*n-3)
	if got != [2]int{n - 2, n - 1} {
		t.Errorf("TwoSum large = %v, want [%d %d]", got, n-2, n-1)
	}
}
