package solution

import (
	"strconv"
	"strings"
)

// RunLengthEncode replaces each maximal run of a repeated byte in s with that
// byte followed by the decimal count of its repetitions, e.g. "aaabbc" becomes
// "a3b2c1". Runs are computed over bytes, not runes. The empty string encodes
// to the empty string.
func RunLengthEncode(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	run := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			run++
			continue
		}
		b.WriteByte(s[i-1])
		b.WriteString(strconv.Itoa(run))
		run = 1
	}
	b.WriteByte(s[len(s)-1])
	b.WriteString(strconv.Itoa(run))
	return b.String()
}
