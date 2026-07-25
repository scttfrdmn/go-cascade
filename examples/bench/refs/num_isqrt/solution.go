package solution

// IntSqrt returns the integer square root of n, i.e. the largest integer r such
// that r*r <= n. It uses only integer arithmetic (no floating point). IntSqrt(0)
// is 0. The argument is a uint64 so that all non-negative values are
// representable and the computation cannot overflow for any valid input.
func IntSqrt(n uint64) uint64 {
	if n < 2 {
		return n
	}
	// Binary search for the largest r with r*r <= n.
	// Upper bound: 2^32 since (2^32)^2 = 2^64 > any uint64.
	var lo, hi uint64 = 1, 1 << 32
	for lo < hi {
		// mid rounded up to converge on the floor.
		mid := lo + (hi-lo+1)/2
		if mid <= n/mid {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}
