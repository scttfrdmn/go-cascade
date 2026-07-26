package solution

// StablePartition returns a new slice in which all elements of s that satisfy
// pred appear first — in their original relative order — followed by all
// elements that do not satisfy pred, also in their original relative order.
//
// The input slice and its backing array are not modified. The partition is
// stable: unlike a two-pointer swap partition, elements within each group keep
// their original ordering.
func StablePartition(s []int, pred func(int) bool) []int {
	out := make([]int, 0, len(s))
	// First pass: elements satisfying the predicate, in original order.
	for _, v := range s {
		if pred(v) {
			out = append(out, v)
		}
	}
	// Second pass: elements not satisfying the predicate, in original order.
	for _, v := range s {
		if !pred(v) {
			out = append(out, v)
		}
	}
	return out
}
