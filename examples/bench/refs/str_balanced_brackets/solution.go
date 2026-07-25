package solution

// IsBalanced reports whether s consists solely of the bracket characters
// (), [] and {} arranged so that every opening bracket is closed by a matching
// bracket in the correct order. Any character other than these six makes the
// string invalid and the result false. The empty string is balanced.
func IsBalanced(s string) bool {
	pairs := map[rune]rune{')': '(', ']': '[', '}': '{'}
	openers := map[rune]bool{'(': true, '[': true, '{': true}
	stack := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case openers[r]:
			stack = append(stack, r)
		case pairs[r] != 0:
			if len(stack) == 0 || stack[len(stack)-1] != pairs[r] {
				return false
			}
			stack = stack[:len(stack)-1]
		default:
			return false
		}
	}
	return len(stack) == 0
}
