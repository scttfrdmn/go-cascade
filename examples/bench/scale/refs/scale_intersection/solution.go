package solution

import "sort"

// Intersection returns the distinct integers present in both a and b, sorted in
// ascending order. When there is no overlap it returns a non-nil empty slice.
// The input slices are not modified.
func Intersection(a, b []int) []int {
	setA := make(map[int]struct{}, len(a))
	for _, v := range a {
		setA[v] = struct{}{}
	}
	seen := make(map[int]struct{})
	result := make([]int, 0)
	for _, v := range b {
		if _, inA := setA[v]; inA {
			if _, dup := seen[v]; !dup {
				seen[v] = struct{}{}
				result = append(result, v)
			}
		}
	}
	sort.Ints(result)
	return result
}
