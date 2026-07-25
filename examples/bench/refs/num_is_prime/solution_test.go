package solution

import "testing"

func TestVBasic(t *testing.T) {
	primes := []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 97}
	for _, p := range primes {
		if !IsPrime(p) {
			t.Fatalf("IsPrime(%d) = false, want true", p)
		}
	}
	composites := []int{4, 6, 8, 9, 10, 12, 15, 21, 25, 27, 49, 100}
	for _, c := range composites {
		if IsPrime(c) {
			t.Fatalf("IsPrime(%d) = true, want false", c)
		}
	}
}

func TestVSmallNonPrimes(t *testing.T) {
	for _, n := range []int{-5, -1, 0, 1} {
		if IsPrime(n) {
			t.Fatalf("IsPrime(%d) = true, want false", n)
		}
	}
}

// referencePrime uses a simple, obviously-correct trial division as an oracle.
func referencePrime(n int) bool {
	if n < 2 {
		return false
	}
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return true
}

func TestHAgainstReference(t *testing.T) {
	for n := -10; n <= 5000; n++ {
		if IsPrime(n) != referencePrime(n) {
			t.Fatalf("IsPrime(%d) = %v, reference = %v", n, IsPrime(n), referencePrime(n))
		}
	}
}

func TestHLargePrimes(t *testing.T) {
	large := []int{104729, 1299709, 15485863, 32452843, 179424673}
	for _, p := range large {
		if !IsPrime(p) {
			t.Fatalf("IsPrime(%d) = false, want true", p)
		}
	}
}

func TestHLargeComposites(t *testing.T) {
	// products of two primes and squares of primes
	comps := []int{104729 * 3, 1299709 * 2, 179424673 + 0*1, 15485863 * 15485863}
	_ = comps
	for _, c := range []int{104729 * 3, 1299709 * 7, 999983 * 999979} {
		if IsPrime(c) {
			t.Fatalf("IsPrime(%d) = true, want false", c)
		}
	}
}

func TestHPerfectSquares(t *testing.T) {
	for _, r := range []int{5, 7, 11, 101, 997} {
		if IsPrime(r * r) {
			t.Fatalf("IsPrime(%d^2=%d) = true, want false", r, r*r)
		}
	}
}
