package solution

import "errors"

// ErrLengthMismatch is returned by Hamming when the two inputs differ in length.
var ErrLengthMismatch = errors.New("hamming: inputs have unequal length")

// Hamming returns the Hamming distance between a and b: the number of byte
// positions at which they differ. The two strings must have equal length; if
// they do not, Hamming returns 0 and ErrLengthMismatch.
func Hamming(a, b string) (int, error) {
	if len(a) != len(b) {
		return 0, ErrLengthMismatch
	}
	dist := 0
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			dist++
		}
	}
	return dist, nil
}
