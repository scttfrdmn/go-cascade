package solution

// ReversedCopy returns a new slice containing the elements of s in reverse
// order. The input slice and its backing array are never modified: the result
// is allocated independently, so callers can safely continue to use s.
func ReversedCopy(s []int) []int {
	out := make([]int, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}
