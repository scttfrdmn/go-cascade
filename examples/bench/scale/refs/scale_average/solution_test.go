package solution

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestVBasic(t *testing.T) {
	if got := Average([]float64{1, 2, 3, 4}); !approx(got, 2.5) {
		t.Fatalf("got %v", got)
	}
}

func TestVSingle(t *testing.T) {
	if got := Average([]float64{42}); !approx(got, 42) {
		t.Fatalf("got %v", got)
	}
}

func TestHNegativesAndZero(t *testing.T) {
	if got := Average([]float64{-1, 1}); !approx(got, 0) {
		t.Fatalf("got %v", got)
	}
	if got := Average([]float64{-3, -6, -9}); !approx(got, -6) {
		t.Fatalf("got %v", got)
	}
}

func TestHFractions(t *testing.T) {
	if got := Average([]float64{0.1, 0.2, 0.3}); !approx(got, 0.2) {
		t.Fatalf("got %v", got)
	}
}

func TestHLarge(t *testing.T) {
	if got := Average([]float64{1e300, 1e300}); !approx(got, 1e300) {
		t.Fatalf("got %v", got)
	}
}
