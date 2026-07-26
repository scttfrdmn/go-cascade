package solution

// Midpoint returns floor((low+high)/2) for non-negative bounds satisfying
// low <= high, without ever overflowing int.
//
// The classic binary-search bug is computing (low+high)/2 directly: when both
// bounds are near the maximum int value their sum overflows and wraps to a
// negative number. The overflow-safe form low + (high-low)/2 computes the same
// midpoint because high-low is always representable when 0 <= low <= high.
//
// Midpoint panics if the arguments are negative or if low > high, since the
// contract requires 0 <= low <= high.
func Midpoint(low, high int) int {
	if low < 0 || high < 0 {
		panic("Midpoint: bounds must be non-negative")
	}
	if low > high {
		panic("Midpoint: low must not exceed high")
	}
	return low + (high-low)/2
}
