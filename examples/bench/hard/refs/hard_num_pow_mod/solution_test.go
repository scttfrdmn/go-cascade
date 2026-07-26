package solution

import (
	"math"
	"math/big"
	"testing"
)

func TestVBasic(t *testing.T) {
	cases := []struct {
		base, exp, m, want int64
	}{
		{2, 10, 1000, 24}, // 1024 mod 1000
		{3, 3, 100, 27},
		{5, 0, 7, 1},  // anything^0 = 1
		{0, 5, 7, 0},  // 0^5 = 0
		{0, 0, 7, 1},  // 0^0 = 1 by convention
		{7, 2, 1, 0},  // mod 1 is always 0
		{2, 5, 13, 6}, // 32 mod 13
		{10, 3, 6, 4}, // 1000 mod 6
	}
	for _, c := range cases {
		if got := PowMod(c.base, c.exp, c.m); got != c.want {
			t.Errorf("PowMod(%d, %d, %d) = %d, want %d", c.base, c.exp, c.m, got, c.want)
		}
	}
}

func TestVSmall(t *testing.T) {
	// Compare against direct integer computation for small values.
	for base := int64(0); base <= 12; base++ {
		for exp := int64(0); exp <= 12; exp++ {
			for _, m := range []int64{1, 2, 3, 7, 97, 1000} {
				want := naivePow(base, exp, m)
				if got := PowMod(base, exp, m); got != want {
					t.Errorf("PowMod(%d,%d,%d)=%d want %d", base, exp, m, got, want)
				}
			}
		}
	}
}

// naivePow computes base^exp mod m for SMALL inputs only, used as an
// independent oracle in tests.
func naivePow(base, exp, m int64) int64 {
	r := int64(1) % m
	for i := int64(0); i < exp; i++ {
		r = (r * base) % m
	}
	return r
}

// TestHLargeModulusOverflow is the core trap. The modulus is close to MaxInt64,
// so intermediate products of two residues (each up to m-1) vastly exceed
// int64. A square-and-multiply implementation using plain int64 multiplication
// silently overflows and returns a wrong answer. The reference oracle here uses
// math/big independently.
func TestHLargeModulusOverflow(t *testing.T) {
	// A large prime-ish modulus near the int64 ceiling.
	m := int64(9223372036854775783) // largest prime < 2^63
	cases := []struct{ base, exp int64 }{
		{2, 62},
		{2, 100},
		{3, 1000000},
		{123456789, 987654321},
		{m - 1, m - 1},
		{9223372036854775782, 2}, // (m-1)^2 mod m == 1
		{7, 9223372036854775806},
	}
	for _, c := range cases {
		want := bigOracle(c.base, c.exp, m)
		if got := PowMod(c.base, c.exp, m); got != want {
			t.Errorf("PowMod(%d, %d, %d) = %d, want %d", c.base, c.exp, m, got, want)
		}
		if got := PowMod(c.base, c.exp, m); got < 0 || got >= m {
			t.Errorf("PowMod(%d, %d, %d) = %d out of range [0,%d)", c.base, c.exp, m, got, m)
		}
	}
}

// TestHSquareOverflow isolates the single-multiply overflow: (m-1)^2 mod m == 1
// for any m > 1, but computing (m-1)*(m-1) in int64 overflows badly when m is
// large.
func TestHSquareOverflow(t *testing.T) {
	m := int64(math.MaxInt64) // 9223372036854775807
	base := m - 1
	// (m-1)^2 = m^2 - 2m + 1 ≡ 1 (mod m).
	if got := PowMod(base, 2, m); got != 1 {
		t.Errorf("PowMod(m-1, 2, m) = %d, want 1", got)
	}
}

// TestHLargeBaseReduced ensures base larger than m is reduced correctly and the
// large-modulus multiply stays exact.
func TestHLargeBaseReduced(t *testing.T) {
	m := int64(9223372036854775783)
	base := int64(math.MaxInt64) // > m, reduces to MaxInt64 - m = 24
	exp := int64(5)
	want := bigOracle(base, exp, m)
	if got := PowMod(base, exp, m); got != want {
		t.Errorf("PowMod(%d, %d, %d) = %d, want %d", base, exp, m, got, want)
	}
}

// bigOracle is an independent big.Int reference for modular exponentiation.
func bigOracle(base, exp, m int64) int64 {
	var r big.Int
	r.Exp(big.NewInt(base), big.NewInt(exp), big.NewInt(m))
	return r.Int64()
}
