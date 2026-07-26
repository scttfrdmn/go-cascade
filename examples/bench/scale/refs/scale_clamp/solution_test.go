package solution

import "testing"

func TestVWithin(t *testing.T) {
	if got := Clamp(5, 0, 10); got != 5 {
		t.Fatalf("got %d want 5", got)
	}
}

func TestVBelow(t *testing.T) {
	if got := Clamp(-3, 0, 10); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestVAbove(t *testing.T) {
	if got := Clamp(42, 0, 10); got != 10 {
		t.Fatalf("got %d want 10", got)
	}
}

func TestHAtLowerBound(t *testing.T) {
	if got := Clamp(0, 0, 10); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestHAtUpperBound(t *testing.T) {
	if got := Clamp(10, 0, 10); got != 10 {
		t.Fatalf("got %d want 10", got)
	}
}

func TestHDegenerateRange(t *testing.T) {
	// lo == hi: everything collapses to that value.
	if got := Clamp(5, 3, 3); got != 3 {
		t.Fatalf("got %d want 3", got)
	}
	if got := Clamp(-100, 3, 3); got != 3 {
		t.Fatalf("got %d want 3", got)
	}
}

func TestHNegativeRange(t *testing.T) {
	if got := Clamp(-5, -10, -1); got != -5 {
		t.Fatalf("got %d want -5", got)
	}
	if got := Clamp(-20, -10, -1); got != -10 {
		t.Fatalf("got %d want -10", got)
	}
	if got := Clamp(0, -10, -1); got != -1 {
		t.Fatalf("got %d want -1", got)
	}
}
