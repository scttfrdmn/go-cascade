package solution

// Caesar returns s with each ASCII letter shifted by k within its own case,
// wrapping around a-z and A-Z. The shift k may be any integer, negative or
// larger than 26; it is normalized modulo 26. All non-letter bytes are left
// unchanged.
func Caesar(s string, k int) string {
	// Normalize the shift into [0, 26) so negative and large shifts behave.
	shift := ((k % 26) + 26) % 26
	if shift == 0 {
		return s
	}
	b := []byte(s)
	for i := range b {
		c := b[i]
		switch {
		case c >= 'a' && c <= 'z':
			b[i] = 'a' + (c-'a'+byte(shift))%26
		case c >= 'A' && c <= 'Z':
			b[i] = 'A' + (c-'A'+byte(shift))%26
		}
	}
	return string(b)
}
