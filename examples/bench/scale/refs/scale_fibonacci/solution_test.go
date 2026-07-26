package solution

import "testing"

func TestVSmall(t *testing.T) {
	want := []uint64{0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55}
	for i, w := range want {
		if got := Fibonacci(i); got != w {
			t.Errorf("Fibonacci(%d) = %d, want %d", i, got, w)
		}
	}
}

func TestVTen(t *testing.T) {
	if got := Fibonacci(10); got != 55 {
		t.Errorf("Fibonacci(10) = %d, want 55", got)
	}
}

func TestHBoundaries(t *testing.T) {
	if got := Fibonacci(0); got != 0 {
		t.Errorf("Fibonacci(0) = %d, want 0", got)
	}
	if got := Fibonacci(1); got != 1 {
		t.Errorf("Fibonacci(1) = %d, want 1", got)
	}
	// Fibonacci(93) is the largest that fits in uint64.
	if got := Fibonacci(93); got != 12200160415121876738 {
		t.Errorf("Fibonacci(93) = %d, want 12200160415121876738", got)
	}
	// Fibonacci(92) for sanity.
	if got := Fibonacci(92); got != 7540113804746346429 {
		t.Errorf("Fibonacci(92) = %d, want 7540113804746346429", got)
	}
}

func TestHOutOfRange(t *testing.T) {
	if got := Fibonacci(-1); got != 0 {
		t.Errorf("Fibonacci(-1) = %d, want 0", got)
	}
	if got := Fibonacci(94); got != 0 {
		t.Errorf("Fibonacci(94) = %d, want 0", got)
	}
}
