package solution

// CountVowels returns the number of ASCII vowels (a, e, i, o, u,
// case-insensitive) in s. Non-ASCII runes and all other characters
// are ignored.
func CountVowels(s string) int {
	count := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
			count++
		}
	}
	return count
}
