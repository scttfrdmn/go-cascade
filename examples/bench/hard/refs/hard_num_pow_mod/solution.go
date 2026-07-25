package solution

import "math/big"

// PowMod computes (base^exp) mod m for non-negative base and exp and a positive
// modulus m. The result is always normalized to the range [0, m).
//
// A naive implementation runs into two independent traps at large inputs:
//
//  1. Computing base^exp first and then taking the modulus overflows int64
//     almost immediately (exponentiation grows astronomically).
//
//  2. Even fast (square-and-multiply) exponentiation multiplies two values that
//     are each up to m-1; when m is large (near the int64 limit) that product
//     overflows int64, corrupting the reduction.
//
// To stay both fast and exact for any int64 inputs we delegate the modular
// arithmetic to math/big, which is part of the standard library. big.Int.Exp
// performs modular exponentiation in O(log exp) big-integer multiplications and
// never overflows.
//
// PowMod panics if m <= 0 or if base or exp is negative, since the result is
// otherwise undefined under this contract. By convention base^0 == 1, so
// PowMod(base, 0, m) returns 1 mod m (which is 0 when m == 1).
func PowMod(base, exp, m int64) int64 {
	if m <= 0 {
		panic("PowMod: modulus must be positive")
	}
	if base < 0 || exp < 0 {
		panic("PowMod: base and exp must be non-negative")
	}

	b := big.NewInt(base)
	e := big.NewInt(exp)
	mod := big.NewInt(m)

	var result big.Int
	result.Exp(b, e, mod) // (b^e) mod mod, non-negative since b, mod > 0

	// The result fits in int64 because it is strictly less than m, which is an
	// int64. big.Int.Exp with a positive modulus returns a value in [0, m).
	return result.Int64()
}
