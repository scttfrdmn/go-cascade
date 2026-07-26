package solution

// Factorial returns n! as a uint64 for 0 <= n <= 20. Factorial(0) is 1.
// For n outside [0, 20] it returns (0, false) because the result either
// is undefined or overflows uint64; otherwise it returns (n!, true).
func Factorial(n int) (uint64, bool) {
	if n < 0 || n > 20 {
		return 0, false
	}
	var result uint64 = 1
	for i := 2; i <= n; i++ {
		result *= uint64(i)
	}
	return result, true
}
