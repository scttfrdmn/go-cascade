package solution

// IsPrime reports whether n is a prime number. Integers less than 2 are not
// prime. The check uses trial division by 2, 3 and then 6k±1 candidates up to
// sqrt(n), using only integer arithmetic.
func IsPrime(n int) bool {
	if n < 2 {
		return false
	}
	if n < 4 {
		return true // 2 and 3
	}
	if n%2 == 0 || n%3 == 0 {
		return false
	}
	// Check 6k-1 and 6k+1 up to sqrt(n). Use i*i <= n to avoid floats.
	for i := 5; i <= n/i; i += 6 {
		if n%i == 0 || n%(i+2) == 0 {
			return false
		}
	}
	return true
}
