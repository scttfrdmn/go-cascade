package solution

import (
	"math"
	"testing"
)

func TestVBasic(t *testing.T) {
	cases := []struct {
		in   uint64
		want uint64
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 2},
		{8, 2},
		{9, 3},
		{15, 3},
		{16, 4},
		{99, 9},
		{100, 10},
		{101, 10},
	}
	for _, c := range cases {
		if got := IntSqrt(c.in); got != c.want {
			t.Fatalf("IntSqrt(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// property: r*r <= n < (r+1)*(r+1)
func checkProperty(t *testing.T, n uint64) {
	t.Helper()
	r := IntSqrt(n)
	if r*r > n {
		t.Fatalf("IntSqrt(%d)=%d but %d*%d=%d > %d", n, r, r, r, r*r, n)
	}
	// Maximality: (r+1)^2 > n. Written via division so it cannot overflow at
	// the top of the uint64 range where (r+1)*(r+1) would wrap to 0.
	if r+1 <= n/(r+1) {
		t.Fatalf("IntSqrt(%d)=%d not maximal", n, r)
	}
}

func TestVProperty(t *testing.T) {
	for n := uint64(0); n <= 10000; n++ {
		checkProperty(t, n)
	}
}

func TestHPerfectSquares(t *testing.T) {
	for r := uint64(0); r <= 100000; r += 1234 {
		n := r * r
		if got := IntSqrt(n); got != r {
			t.Fatalf("IntSqrt(%d) = %d, want %d", n, got, r)
		}
	}
}

func TestHLargeValues(t *testing.T) {
	// Compare against a trusted computation for large numbers.
	big := []uint64{
		1 << 40,
		1 << 50,
		1 << 62,
		math.MaxUint64,
		math.MaxUint64 - 1,
		(1<<32)*(1<<32) - 1, // = 2^64 - 1 == MaxUint64 too
	}
	for _, n := range big {
		checkProperty(t, n)
	}
}

func TestHMaxUint64(t *testing.T) {
	// floor(sqrt(2^64-1)) == 2^32 - 1 == 4294967295
	if got := IntSqrt(math.MaxUint64); got != 4294967295 {
		t.Fatalf("IntSqrt(MaxUint64) = %d, want 4294967295", got)
	}
}

func TestHAroundPowersOfTwo(t *testing.T) {
	for p := uint(1); p < 63; p++ {
		n := uint64(1) << p
		checkProperty(t, n)
		if n > 0 {
			checkProperty(t, n-1)
		}
		checkProperty(t, n+1)
	}
}
