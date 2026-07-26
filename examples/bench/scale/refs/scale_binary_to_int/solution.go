package solution

// BinaryToInt parses s as an unsigned binary number and returns its value. The
// string must be non-empty and consist solely of the characters '0' and '1';
// any other character (or an empty string) makes it invalid, in which case the
// returned value is 0 and ok is false.
func BinaryToInt(s string) (value uint64, ok bool) {
	if len(s) == 0 {
		return 0, false
	}
	var v uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '0' && c != '1' {
			return 0, false
		}
		v = v<<1 | uint64(c-'0')
	}
	return v, true
}
