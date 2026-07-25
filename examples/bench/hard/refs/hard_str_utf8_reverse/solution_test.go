package solution

import "testing"

// naiveByteReverse reverses a string byte-by-byte. This is the classic buggy
// implementation that corrupts multi-byte UTF-8 sequences. It exists only so
// the hidden test can prove the correct solution differs from it.
func naiveByteReverse(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

func TestVReverseASCII(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"a":       "a",
		"ab":      "ba",
		"abc":     "cba",
		"hello":   "olleh",
		"racecar": "racecar",
	}
	for in, want := range cases {
		if got := ReverseRunes(in); got != want {
			t.Errorf("ReverseRunes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVReverseTwice(t *testing.T) {
	// Reversing twice must yield the original for any input.
	for _, s := range []string{"", "x", "hello world", "12345"} {
		if got := ReverseRunes(ReverseRunes(s)); got != s {
			t.Errorf("ReverseRunes(ReverseRunes(%q)) = %q, want original", s, got)
		}
	}
}

func TestHReverseUnicode(t *testing.T) {
	cases := map[string]string{
		"héllo": "olléh",
		"日本語":   "語本日",
		"café":  "éfac",
		"a😀b":   "b😀a",
		"🇺🇸":    "🇸🇺", // two regional-indicator runes
		"Åland": "dnalÅ",
	}
	for in, want := range cases {
		if got := ReverseRunes(in); got != want {
			t.Errorf("ReverseRunes(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestHReverseBeatsNaive is THE TRAP: the correct rune-aware reverse must
// differ from a byte-level reverse for multi-byte input, and every rune in
// the result must be valid (no corrupted bytes producing U+FFFD from a valid
// input).
func TestHReverseBeatsNaive(t *testing.T) {
	for _, s := range []string{"héllo", "日本語", "café", "a😀b"} {
		correct := ReverseRunes(s)
		naive := naiveByteReverse(s)
		if correct == naive {
			t.Errorf("ReverseRunes(%q) = %q must differ from byte-reverse %q", s, correct, naive)
		}
		// The correct output must contain the same multiset of runes as input.
		if !sameRuneMultiset(s, correct) {
			t.Errorf("ReverseRunes(%q) = %q corrupted runes", s, correct)
		}
	}
}

func sameRuneMultiset(a, b string) bool {
	m := map[rune]int{}
	for _, r := range a {
		m[r]++
	}
	for _, r := range b {
		m[r]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}
