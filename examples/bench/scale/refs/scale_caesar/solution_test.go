package solution

import "testing"

func TestVBasic(t *testing.T) {
	if got := Caesar("abc", 1); got != "bcd" {
		t.Fatalf("Caesar(abc,1)=%q", got)
	}
	if got := Caesar("xyz", 3); got != "abc" {
		t.Fatalf("Caesar(xyz,3)=%q", got)
	}
	if got := Caesar("ABC", 2); got != "CDE" {
		t.Fatalf("Caesar(ABC,2)=%q", got)
	}
}

func TestVMixed(t *testing.T) {
	if got := Caesar("Hello, World!", 1); got != "Ifmmp, Xpsme!" {
		t.Fatalf("got %q", got)
	}
}

func TestHNegative(t *testing.T) {
	if got := Caesar("bcd", -1); got != "abc" {
		t.Fatalf("neg shift: %q", got)
	}
	// -27 mod 26 == -1 mod 26 == 25 shift; equivalent to shifting by 25.
	if got := Caesar("a", -27); got != "z" {
		t.Fatalf("Caesar(a,-27)=%q", got)
	}
}

func TestHLargeAndZero(t *testing.T) {
	if got := Caesar("abc", 0); got != "abc" {
		t.Fatalf("zero shift: %q", got)
	}
	if got := Caesar("abc", 26); got != "abc" {
		t.Fatalf("26 shift: %q", got)
	}
	if got := Caesar("abc", 52); got != "abc" {
		t.Fatalf("52 shift: %q", got)
	}
	if got := Caesar("abc", 27); got != "bcd" {
		t.Fatalf("27 shift: %q", got)
	}
}

func TestHEmptyAndNonLetters(t *testing.T) {
	if got := Caesar("", 5); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := Caesar("123 !@#\n", 13); got != "123 !@#\n" {
		t.Fatalf("non-letters: %q", got)
	}
}

func TestHWrapBoundaries(t *testing.T) {
	if got := Caesar("z", 1); got != "a" {
		t.Fatalf("z+1=%q", got)
	}
	if got := Caesar("Z", 1); got != "A" {
		t.Fatalf("Z+1=%q", got)
	}
	if got := Caesar("a", -1); got != "z" {
		t.Fatalf("a-1=%q", got)
	}
}
