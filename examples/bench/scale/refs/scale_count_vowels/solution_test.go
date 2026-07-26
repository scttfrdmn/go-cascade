package solution

import (
	"strings"
	"testing"
)

func TestVCountVowels(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"hello", 2},
		{"HELLO", 2},
		{"AEIOU", 5},
		{"aeiou", 5},
		{"xyz", 0},
		{"Programming", 3},
	}
	for _, c := range cases {
		if got := CountVowels(c.in); got != c.want {
			t.Errorf("CountVowels(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestHCountVowels(t *testing.T) {
	// Empty string.
	if got := CountVowels(""); got != 0 {
		t.Errorf("CountVowels(%q) = %d, want 0", "", got)
	}
	// 'y' is not a vowel here.
	if got := CountVowels("y Y rhythm"); got != 0 {
		t.Errorf("CountVowels(y) = %d, want 0", got)
	}
	// Non-ASCII must be ignored, even bytes that resemble vowels.
	// "café résumé" has ASCII vowels 'a' and 'u' only; every 'e' is é.
	if got := CountVowels("café résumé"); got != 2 {
		t.Errorf("CountVowels(accented) = %d, want 2", got)
	}
	// Large input: 100000 'a' characters.
	big := strings.Repeat("a", 100000)
	if got := CountVowels(big); got != 100000 {
		t.Errorf("CountVowels(big) = %d, want 100000", got)
	}
	// Mixed with punctuation and digits.
	if got := CountVowels("a1e2i3o4u5!"); got != 5 {
		t.Errorf("CountVowels(mixed) = %d, want 5", got)
	}
}
