package solution

// RotateLeft returns a new slice equal to nums rotated left by k positions.
// k may be zero or exceed the length of nums; it is reduced modulo the length.
// The input slice is never modified. An empty (or nil) input yields an empty,
// non-nil slice.
func RotateLeft(nums []int, k int) []int {
	n := len(nums)
	out := make([]int, n)
	if n == 0 {
		return out
	}
	shift := ((k % n) + n) % n
	for i := 0; i < n; i++ {
		out[i] = nums[(i+shift)%n]
	}
	return out
}
