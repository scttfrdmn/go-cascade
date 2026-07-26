package solution

// IsPalindrome reports whether s reads the same forwards and backwards
// when only ASCII alphanumeric characters are considered and letter case
// is ignored. All other characters (spaces, punctuation, non-ASCII bytes)
// are skipped. The empty string is a palindrome.
func IsPalindrome(s string) bool {
	i, j := 0, len(s)-1
	for i < j {
		for i < j && !isAlnum(s[i]) {
			i++
		}
		for i < j && !isAlnum(s[j]) {
			j--
		}
		if i < j {
			if toLower(s[i]) != toLower(s[j]) {
				return false
			}
			i++
			j--
		}
	}
	return true
}

func isAlnum(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z')
}

func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
