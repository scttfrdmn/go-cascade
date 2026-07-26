package solution

// CountWords counts the words in s, where a word is a maximal run of
// characters that are not the ASCII space ' '. Leading, trailing, and
// repeated spaces do not create empty words. An empty or all-space
// string yields 0. Only the ASCII space ' ' is treated as a separator;
// tabs, newlines, and other characters are part of words.
func CountWords(s string) int {
	count := 0
	inWord := false
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}
