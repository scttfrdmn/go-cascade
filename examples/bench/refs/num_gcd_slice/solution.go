package solution

// GCDSlice returns the greatest common divisor of all integers in xs. The slice
// must be non-empty and every element must be a positive integer; the function
// panics on an empty slice because a GCD is undefined there. For a single
// element the result is that element.
func GCDSlice(xs []int) int {
	if len(xs) == 0 {
		panic("GCDSlice: empty slice has no GCD")
	}
	g := xs[0]
	for _, v := range xs[1:] {
		g = gcd(g, v)
		if g == 1 {
			return 1
		}
	}
	return g
}

// gcd returns the greatest common divisor of a and b using the Euclidean
// algorithm. Inputs are assumed non-negative.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
