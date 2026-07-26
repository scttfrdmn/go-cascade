package solution

// Clamp constrains x to the inclusive range [lo, hi], where lo <= hi.
// It returns lo if x < lo, hi if x > hi, and x otherwise.
func Clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
