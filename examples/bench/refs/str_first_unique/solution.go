package solution

// FirstUniqueByte returns the index of the first byte in s that appears
// exactly once. If every byte repeats, or s is empty, it returns -1.
func FirstUniqueByte(s string) int {
	var counts [256]int
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	for i := 0; i < len(s); i++ {
		if counts[s[i]] == 1 {
			return i
		}
	}
	return -1
}
