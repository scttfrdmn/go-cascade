package solution

// MaxWindowSum returns the maximum sum of any k consecutive elements of
// xs. The caller must supply 1 <= k <= len(xs); for k outside that range
// (including an empty slice) it returns 0.
func MaxWindowSum(xs []int, k int) int {
	if k < 1 || k > len(xs) {
		return 0
	}
	// Sum of the first window.
	sum := 0
	for i := 0; i < k; i++ {
		sum += xs[i]
	}
	best := sum
	// Slide the window: add the entering element, drop the leaving one.
	for i := k; i < len(xs); i++ {
		sum += xs[i] - xs[i-k]
		if sum > best {
			best = sum
		}
	}
	return best
}
