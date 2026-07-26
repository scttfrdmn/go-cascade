package solution

import "math"

// ReverseInt reverses the decimal digits of the 32-bit signed integer x,
// preserving its sign. For example 123 becomes 321 and -120 becomes -21.
// If the reversed value falls outside the int32 range it returns 0.
func ReverseInt(x int32) int32 {
	var rev int64
	n := int64(x)
	for n != 0 {
		rev = rev*10 + n%10
		n /= 10
	}
	if rev < math.MinInt32 || rev > math.MaxInt32 {
		return 0
	}
	return int32(rev)
}
