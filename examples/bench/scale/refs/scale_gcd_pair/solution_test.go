package solution

import "testing"

func TestVBasic(t *testing.T) {
	if got := GCD(12, 18); got != 6 {
		t.Errorf("GCD(12,18) = %d, want 6", got)
	}
	if got := GCD(48, 36); got != 12 {
		t.Errorf("GCD(48,36) = %d, want 12", got)
	}
}

func TestVCoprime(t *testing.T) {
	if got := GCD(13, 7); got != 1 {
		t.Errorf("GCD(13,7) = %d, want 1", got)
	}
}

func TestHZero(t *testing.T) {
	if got := GCD(0, 5); got != 5 {
		t.Errorf("GCD(0,5) = %d, want 5", got)
	}
	if got := GCD(5, 0); got != 5 {
		t.Errorf("GCD(5,0) = %d, want 5", got)
	}
	if got := GCD(0, 0); got != 0 {
		t.Errorf("GCD(0,0) = %d, want 0", got)
	}
}

func TestHEqual(t *testing.T) {
	if got := GCD(9, 9); got != 9 {
		t.Errorf("GCD(9,9) = %d, want 9", got)
	}
}

func TestHLargeAndPrime(t *testing.T) {
	// Large coprime-ish values.
	if got := GCD(1000000007, 998244353); got != 1 {
		t.Errorf("GCD(1000000007,998244353) = %d, want 1", got)
	}
	// One divides the other.
	if got := GCD(1000000, 250); got != 250 {
		t.Errorf("GCD(1000000,250) = %d, want 250", got)
	}
}
