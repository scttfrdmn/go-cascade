package solution

// SearchInsertionIndex returns the leftmost index at which target could be
// inserted into the ascending-sorted slice nums to keep it sorted. If target
// is present, the returned index is that of the leftmost matching element.
// For an empty slice the result is 0.
func SearchInsertionIndex(nums []int, target int) int {
	lo, hi := 0, len(nums)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if nums[mid] < target {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}
