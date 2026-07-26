package solution

// RunningMax returns a slice of the same length as values in which
// element i is the maximum of values[0..i] inclusive. The result is
// always non-nil, even for empty or nil input. The input is not
// modified.
func RunningMax(values []int) []int {
	out := make([]int, len(values))
	for i, v := range values {
		if i == 0 || v > out[i-1] {
			out[i] = v
		} else {
			out[i] = out[i-1]
		}
	}
	return out
}
