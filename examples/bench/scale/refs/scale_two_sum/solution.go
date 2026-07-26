package solution

// TwoSum returns the indices of the two distinct elements of nums that
// sum to target, with the smaller index first, as a [2]int. The problem
// guarantees exactly one such pair exists. If no pair is found (which
// should not happen for valid inputs) it returns [2]int{-1, -1}.
func TwoSum(nums []int, target int) [2]int {
	// seen maps a value to the earliest index at which it appeared.
	seen := make(map[int]int, len(nums))
	for i, v := range nums {
		if j, ok := seen[target-v]; ok {
			return [2]int{j, i}
		}
		if _, ok := seen[v]; !ok {
			seen[v] = i
		}
	}
	return [2]int{-1, -1}
}
