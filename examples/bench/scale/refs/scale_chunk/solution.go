package solution

// Chunk splits values into consecutive chunks of at most n elements,
// preserving order. n must be >= 1; for n < 1 the elements are returned
// as a single chunk to avoid a degenerate infinite split. The final
// chunk may be shorter than n. The result is always non-nil, even for
// empty input. The input slice is not modified.
func Chunk(values []int, n int) [][]int {
	out := make([][]int, 0)
	if len(values) == 0 {
		return out
	}
	if n < 1 {
		chunk := make([]int, len(values))
		copy(chunk, values)
		return append(out, chunk)
	}
	for i := 0; i < len(values); i += n {
		end := i + n
		if end > len(values) {
			end = len(values)
		}
		chunk := make([]int, end-i)
		copy(chunk, values[i:end])
		out = append(out, chunk)
	}
	return out
}
