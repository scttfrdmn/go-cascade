package solution

// SumDigits returns the sum of the decimal digits of the non-negative
// integer n. SumDigits(0) is 0.
func SumDigits(n uint64) int {
	sum := 0
	for n > 0 {
		sum += int(n % 10)
		n /= 10
	}
	return sum
}
