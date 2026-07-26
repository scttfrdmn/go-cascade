package solution

// DedupSortedInPlace removes consecutive duplicates from an ascending-sorted
// slice in place and returns the prefix length k. After the call, the first k
// elements of s hold the distinct values in ascending order. The remaining
// elements of s (from index k onward) are left with their original contents and
// should be considered scratch; callers typically use s[:k].
//
// The operation is performed in place: no new backing array is allocated, and
// the returned length k is the number of distinct values (0 for an empty input).
func DedupSortedInPlace(s []int) int {
	if len(s) == 0 {
		return 0
	}
	k := 1
	for i := 1; i < len(s); i++ {
		if s[i] != s[k-1] {
			s[k] = s[i]
			k++
		}
	}
	return k
}
