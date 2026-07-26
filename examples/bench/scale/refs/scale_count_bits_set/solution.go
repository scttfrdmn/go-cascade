package solution

// CountBits returns a slice of length n+1 where element i holds the
// number of set bits (popcount) in the integer i, for 0 <= i <= n.
// n must be non-negative; for negative n an empty slice is returned.
func CountBits(n int) []int {
	if n < 0 {
		return []int{}
	}
	result := make([]int, n+1)
	for i := 1; i <= n; i++ {
		// i&(i-1) clears the lowest set bit, so i has one more set
		// bit than that value.
		result[i] = result[i&(i-1)] + 1
	}
	return result
}
