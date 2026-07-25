package solution

import (
	"strings"
	"testing"
)

func TestVWordCountBasic(t *testing.T) {
	cases := map[string]int{
		"hello world":         2,
		"one":                 1,
		"a b c":               3,
		"the quick brown fox": 4,
	}
	for in, want := range cases {
		if got := WordCount(in); got != want {
			t.Errorf("WordCount(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestVWordCountEmpty(t *testing.T) {
	if got := WordCount(""); got != 0 {
		t.Errorf("WordCount(%q) = %d, want 0", "", got)
	}
}

// TestHWordCountWhitespace is THE TRAP: leading/trailing/repeated separators,
// mixed Unicode whitespace, and all-whitespace strings. A naive
// strings.Split(s, " ") overcounts here and misses non-space whitespace.
func TestHWordCountWhitespace(t *testing.T) {
	cases := map[string]int{
		"   leading":         1,
		"trailing   ":        1,
		"  both  sides  ":    2,
		"a\tb\nc":            3, // tab and newline separators
		"a b":                2, // non-breaking space U+00A0
		"one   two    three": 3, // repeated ASCII spaces
		"  ":                 0, // all non-breaking spaces
		" \t\n\r ":           0, // all whitespace, mixed
		"":                   0,
		"word word word":     3, // em space U+2003
		"日本語 テスト":            2, // multibyte words
	}
	for in, want := range cases {
		if got := WordCount(in); got != want {
			t.Errorf("WordCount(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestHWordCountBeatsSplit proves the correct implementation differs from the
// naive strings.Split(s, " ") approach on trap inputs.
func TestHWordCountBeatsSplit(t *testing.T) {
	traps := []string{"  both  sides  ", "a\tb\nc", "a b", "   leading"}
	for _, s := range traps {
		naive := len(strings.Split(s, " "))
		if got := WordCount(s); got == naive {
			t.Errorf("WordCount(%q) = %d must differ from naive split count %d", s, got, naive)
		}
	}
}
