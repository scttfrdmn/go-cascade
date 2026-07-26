package solution

import (
	"strings"
	"testing"
)

func TestVIsPalindrome(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"racecar", true},
		{"A man, a plan, a canal: Panama", true},
		{"hello", false},
		{"Was it a car or a cat I saw?", true},
		{"abc", false},
		{"a", true},
	}
	for _, c := range cases {
		if got := IsPalindrome(c.in); got != c.want {
			t.Errorf("IsPalindrome(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestHIsPalindrome(t *testing.T) {
	// Empty string is a palindrome.
	if !IsPalindrome("") {
		t.Errorf("IsPalindrome(\"\") = false, want true")
	}
	// String of only non-alnum chars reduces to empty -> palindrome.
	if !IsPalindrome(".,!?  ") {
		t.Errorf("IsPalindrome(punctuation only) = false, want true")
	}
	// Case folding across ends.
	if !IsPalindrome("Aa") {
		t.Errorf("IsPalindrome(\"Aa\") = false, want true")
	}
	// Digits participate; letters fold case: "0P0" -> "0p0" is a palindrome.
	if !IsPalindrome("0P0") {
		t.Errorf("IsPalindrome(\"0P0\") = false, want true")
	}
	if !IsPalindrome("12321") {
		t.Errorf("IsPalindrome(\"12321\") = false, want true")
	}
	if IsPalindrome("10") {
		t.Errorf("IsPalindrome(\"10\") = true, want false")
	}
	// Non-ASCII bytes are skipped; the alnum skeleton "aa" is a palindrome.
	if !IsPalindrome("aéa") {
		t.Errorf("IsPalindrome(non-ascii wrapped) = false, want true")
	}
	// Large palindrome: construct s + reverse(s), guaranteed symmetric.
	half := strings.Repeat("Go1.26!", 10000)
	rev := reverseString(half)
	if !IsPalindrome(half + rev) {
		t.Errorf("IsPalindrome(large constructed) = false, want true")
	}
	// Large non-palindrome.
	if IsPalindrome(strings.Repeat("a", 99999) + "b") {
		t.Errorf("IsPalindrome(large a...b) = true, want false")
	}
}

func reverseString(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}
