package solution

// MaxSubarraySum returns the maximum sum of any non-empty contiguous
// subarray of nums using Kadane's algorithm. nums must be non-empty;
// for an empty slice it returns 0. All-negative inputs are handled
// correctly: the result is the largest (least negative) element.
func MaxSubarraySum(nums []int) int {
	if len(nums) == 0 {
		return 0
	}
	best := nums[0]
	cur := nums[0]
	for _, v := range nums[1:] {
		if cur+v < v {
			cur = v
		} else {
			cur = cur + v
		}
		if cur > best {
			best = cur
		}
	}
	return best
}
