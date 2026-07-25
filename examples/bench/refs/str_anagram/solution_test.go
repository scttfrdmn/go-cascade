package solution

import "testing"

func TestVBasic(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"listen", "silent", true},
		{"Listen", "Silent", true},
		{"dormitory", "dirty room", true},
		{"hello", "world", false},
		{"abc", "abcd", false},
		{"", "", true},
	}
	for _, c := range cases {
		if got := AreAnagrams(c.a, c.b); got != c.want {
			t.Fatalf("AreAnagrams(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestVCaseInsensitive(t *testing.T) {
	if !AreAnagrams("Tea", "EAT") {
		t.Fatal("Tea/EAT should be anagrams")
	}
}

func TestVSpacesIgnored(t *testing.T) {
	if !AreAnagrams("a b c", "cba") {
		t.Fatal("spaces should be ignored")
	}
}

func TestHPunctuationSignificant(t *testing.T) {
	if AreAnagrams("a!", "a") {
		t.Fatal("punctuation is significant")
	}
	if !AreAnagrams("a!b", "b!a") {
		t.Fatal("same punctuation multiset should match")
	}
}

func TestHDigitsSignificant(t *testing.T) {
	if !AreAnagrams("a1b2", "2b1a") {
		t.Fatal("digits should be treated as significant runes")
	}
	if AreAnagrams("a1", "a2") {
		t.Fatal("different digits are not anagrams")
	}
}

func TestHEmptyAndSpacesOnly(t *testing.T) {
	if !AreAnagrams("   ", "") {
		t.Fatal("spaces-only vs empty should be anagrams")
	}
	if !AreAnagrams("  a  ", "a") {
		t.Fatal("surrounding spaces ignored")
	}
}

func TestHUnicode(t *testing.T) {
	if !AreAnagrams("Café", "éfac") {
		t.Fatal("unicode case folding and reorder should match")
	}
	if AreAnagrams("é", "e") {
		t.Fatal("accented and plain letters differ")
	}
}

func TestHCountsMatter(t *testing.T) {
	if AreAnagrams("aab", "abb") {
		t.Fatal("multiset counts must match")
	}
}
