package solution

import "sort"

// UniqueSorted returns the distinct integers of values in ascending
// order. The result is always non-nil, even for empty or nil input.
// The input slice is not modified.
func UniqueSorted(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
