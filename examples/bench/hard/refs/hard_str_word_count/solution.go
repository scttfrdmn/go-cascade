package solution

import "unicode"

// WordCount returns the number of words in s, where a word is a maximal run of
// non-whitespace runes. Whitespace is any Unicode space as defined by
// unicode.IsSpace (spaces, tabs, newlines, the non-breaking space U+00A0, and
// others). Leading, trailing, and repeated separators produce no empty words;
// an empty or all-whitespace string yields 0.
func WordCount(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}
