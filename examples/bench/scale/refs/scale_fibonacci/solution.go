package solution

// Fibonacci returns the nth Fibonacci number as a uint64, where
// Fibonacci(0) == 0 and Fibonacci(1) == 1. It is defined for
// 0 <= n <= 93; Fibonacci(93) is the largest value that fits in a
// uint64. For n outside that range the result is undefined and 0 is
// returned.
func Fibonacci(n int) uint64 {
	if n < 0 || n > 93 {
		return 0
	}
	var a, b uint64 = 0, 1
	for range n {
		a, b = b, a+b
	}
	return a
}
