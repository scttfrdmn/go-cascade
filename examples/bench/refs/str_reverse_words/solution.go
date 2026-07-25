package solution

import "strings"

// ReverseWords takes a string of words separated by single spaces and returns
// the words in reverse order, joined by single spaces, with no leading or
// trailing whitespace. The empty string is returned unchanged.
func ReverseWords(s string) string {
	if s == "" {
		return ""
	}
	words := strings.Split(s, " ")
	for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
		words[i], words[j] = words[j], words[i]
	}
	return strings.Join(words, " ")
}
