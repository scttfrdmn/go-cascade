package solution

// ReverseRunes returns s with its Unicode code points (runes) in reverse
// order. Multi-byte UTF-8 sequences are preserved intact rather than being
// reversed byte-by-byte (which would corrupt them). An empty string yields
// an empty string.
func ReverseRunes(s string) string {
	// Convert to a slice of runes so multi-byte characters stay together.
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
