package solution

// Flatten concatenates the inner slices of nested into a single slice,
// preserving order. Nil or empty inner slices contribute nothing. The
// result is always non-nil, even when nested is empty or contains no
// elements. The input is not modified.
func Flatten(nested [][]int) []int {
	out := make([]int, 0)
	for _, inner := range nested {
		out = append(out, inner...)
	}
	return out
}
