package solution

// MostFrequent returns the most frequently occurring integer in the non-empty
// slice nums. When several values share the highest count, the smallest such
// value is returned. Calling MostFrequent with an empty slice returns 0 and
// false; otherwise it returns the winning value and true.
func MostFrequent(nums []int) (int, bool) {
	if len(nums) == 0 {
		return 0, false
	}
	counts := make(map[int]int, len(nums))
	for _, v := range nums {
		counts[v]++
	}
	best := nums[0]
	bestCount := 0
	for v, c := range counts {
		if c > bestCount || (c == bestCount && v < best) {
			best = v
			bestCount = c
		}
	}
	return best, true
}
