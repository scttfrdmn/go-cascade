package solution

// DigitalRoot repeatedly sums the decimal digits of the non-negative
// integer n until a single decimal digit remains, and returns it.
// The digital root of 0 is 0.
func DigitalRoot(n uint64) int {
	for n >= 10 {
		var sum uint64
		for n > 0 {
			sum += n % 10
			n /= 10
		}
		n = sum
	}
	return int(n)
}
