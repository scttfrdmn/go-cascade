package solution

// MoveZeros returns a new slice in which every zero from nums has been moved to
// the end while the relative order of the non-zero elements is preserved. The
// input slice is not modified. For an empty or nil input a non-nil empty slice
// is returned.
func MoveZeros(nums []int) []int {
	result := make([]int, len(nums))
	i := 0
	for _, v := range nums {
		if v != 0 {
			result[i] = v
			i++
		}
	}
	// Remaining entries are already the zero value 0, filling the tail.
	return result
}
