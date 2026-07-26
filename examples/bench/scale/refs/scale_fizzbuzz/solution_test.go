package solution

import (
	"strconv"
	"testing"
)

func TestVFizzBuzz(t *testing.T) {
	got := FizzBuzz(15)
	want := []string{
		"1", "2", "Fizz", "4", "Buzz",
		"Fizz", "7", "8", "Fizz", "Buzz",
		"11", "Fizz", "13", "14", "FizzBuzz",
	}
	if len(got) != len(want) {
		t.Fatalf("FizzBuzz(15) len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FizzBuzz(15)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVFizzBuzzShort(t *testing.T) {
	got := FizzBuzz(1)
	if len(got) != 1 || got[0] != "1" {
		t.Errorf("FizzBuzz(1) = %v, want [1]", got)
	}
}

func TestHFizzBuzz(t *testing.T) {
	// n == 0 yields empty, non-nil slice.
	got := FizzBuzz(0)
	if got == nil {
		t.Errorf("FizzBuzz(0) is nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("FizzBuzz(0) len = %d, want 0", len(got))
	}
	// Negative n treated as empty.
	if g := FizzBuzz(-5); len(g) != 0 {
		t.Errorf("FizzBuzz(-5) len = %d, want 0", len(g))
	}

	// Large n: verify length and a full re-derivation.
	const n = 10000
	big := FizzBuzz(n)
	if len(big) != n {
		t.Fatalf("FizzBuzz(%d) len = %d, want %d", n, len(big), n)
	}
	for i := 1; i <= n; i++ {
		var want string
		switch {
		case i%15 == 0:
			want = "FizzBuzz"
		case i%3 == 0:
			want = "Fizz"
		case i%5 == 0:
			want = "Buzz"
		default:
			want = strconv.Itoa(i)
		}
		if big[i-1] != want {
			t.Fatalf("FizzBuzz(%d)[%d] = %q, want %q", n, i-1, big[i-1], want)
		}
	}
}
