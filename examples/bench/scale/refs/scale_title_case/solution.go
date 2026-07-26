package solution

import "strings"

// TitleCase returns s with the first ASCII letter of each
// space-separated word upper-cased and every other ASCII letter
// lower-cased. Words are separated by single spaces and the input has
// no leading or trailing whitespace. Non-letter characters are left
// unchanged.
func TitleCase(s string) string {
	words := strings.Split(s, " ")
	for i, w := range words {
		words[i] = titleWord(w)
	}
	return strings.Join(words, " ")
}

func titleWord(w string) string {
	b := []byte(w)
	for i := range b {
		if i == 0 {
			b[i] = toUpper(b[i])
		} else {
			b[i] = toLower(b[i])
		}
	}
	return string(b)
}

func toUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
