package solution

import "strconv"

// FizzBuzz returns a slice of length n where the element at index i
// (0-based) describes the position i+1 (1-based): "FizzBuzz" when the
// position is divisible by both 3 and 5, "Fizz" when divisible by 3,
// "Buzz" when divisible by 5, otherwise the decimal string of the
// position. For n <= 0 it returns an empty, non-nil slice.
func FizzBuzz(n int) []string {
	if n < 0 {
		n = 0
	}
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		switch {
		case i%15 == 0:
			out = append(out, "FizzBuzz")
		case i%3 == 0:
			out = append(out, "Fizz")
		case i%5 == 0:
			out = append(out, "Buzz")
		default:
			out = append(out, strconv.Itoa(i))
		}
	}
	return out
}
