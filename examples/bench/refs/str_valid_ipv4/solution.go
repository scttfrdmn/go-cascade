package solution

import "strings"

// IsValidIPv4 reports whether s is a valid dotted-decimal IPv4 address:
// exactly four decimal octets separated by dots, each in the range
// 0..255, with no leading zeros (except the single digit "0"), and no
// extra characters.
func IsValidIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if !validOctet(p) {
			return false
		}
	}
	return true
}

// validOctet reports whether p is a canonical decimal octet 0..255.
func validOctet(p string) bool {
	n := len(p)
	if n == 0 || n > 3 {
		return false
	}
	// Reject leading zeros unless the octet is exactly "0".
	if p[0] == '0' && n > 1 {
		return false
	}
	value := 0
	for i := 0; i < n; i++ {
		c := p[i]
		if c < '0' || c > '9' {
			return false
		}
		value = value*10 + int(c-'0')
	}
	return value <= 255
}
