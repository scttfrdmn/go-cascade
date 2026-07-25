package solution

// PopCount returns the number of set bits (ones) in the binary
// representation of the non-negative integer n.
func PopCount(n uint64) int {
	count := 0
	for n != 0 {
		n &= n - 1 // clear the lowest set bit
		count++
	}
	return count
}
