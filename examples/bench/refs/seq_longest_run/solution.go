package solution

// LongestIncreasingRun returns the length of the longest strictly increasing
// contiguous run in xs. A run is a maximal stretch of consecutive elements
// where each element is strictly greater than the one before it. For an empty
// slice the result is 0.
func LongestIncreasingRun(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	best := 1
	cur := 1
	for i := 1; i < len(xs); i++ {
		if xs[i] > xs[i-1] {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 1
		}
	}
	return best
}
