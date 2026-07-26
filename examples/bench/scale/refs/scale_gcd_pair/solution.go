package solution

// GCD returns the greatest common divisor of two non-negative integers
// a and b, which must not both be zero. It follows the convention
// gcd(0, x) == x. If both are zero it returns 0.
func GCD(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
