package solution

import "unicode"

// AreAnagrams reports whether a and b are anagrams of each other. Comparison is
// case-insensitive and spaces are ignored; every other character (letters,
// digits, punctuation, other Unicode) is significant. Two strings are anagrams
// when, after removing spaces and folding case, they contain the same multiset
// of runes.
func AreAnagrams(a, b string) bool {
	counts := make(map[rune]int)
	for _, r := range a {
		if r == ' ' {
			continue
		}
		counts[unicode.ToLower(r)]++
	}
	for _, r := range b {
		if r == ' ' {
			continue
		}
		counts[unicode.ToLower(r)]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}
