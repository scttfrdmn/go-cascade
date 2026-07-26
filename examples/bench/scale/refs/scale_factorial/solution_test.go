package solution

import "testing"

func TestVFactorial(t *testing.T) {
	cases := []struct {
		n    int
		want uint64
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 6},
		{5, 120},
		{10, 3628800},
	}
	for _, c := range cases {
		got, ok := Factorial(c.n)
		if !ok {
			t.Errorf("Factorial(%d) ok = false, want true", c.n)
			continue
		}
		if got != c.want {
			t.Errorf("Factorial(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestHFactorial(t *testing.T) {
	// Upper boundary: 20! fits in uint64 (2432902008176640000).
	got, ok := Factorial(20)
	if !ok || got != 2432902008176640000 {
		t.Errorf("Factorial(20) = (%d, %v), want (2432902008176640000, true)", got, ok)
	}
	// Just past the boundary must fail.
	if _, ok := Factorial(21); ok {
		t.Errorf("Factorial(21) ok = true, want false")
	}
	// Negative input must fail.
	if _, ok := Factorial(-1); ok {
		t.Errorf("Factorial(-1) ok = true, want false")
	}
	// Large out-of-range value.
	if _, ok := Factorial(1000000); ok {
		t.Errorf("Factorial(1e6) ok = true, want false")
	}
	// Verify the recurrence n! == n * (n-1)! within range.
	prev, _ := Factorial(0)
	for n := 1; n <= 20; n++ {
		cur, ok := Factorial(n)
		if !ok {
			t.Fatalf("Factorial(%d) unexpectedly failed", n)
		}
		if cur != uint64(n)*prev {
			t.Fatalf("Factorial(%d) = %d, want %d", n, cur, uint64(n)*prev)
		}
		prev = cur
	}
}
