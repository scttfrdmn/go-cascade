package solution

// RomanToInt converts a valid Roman numeral string composed of the uppercase
// symbols I, V, X, L, C, D and M, using standard subtractive notation, to its
// integer value. The empty string converts to 0.
func RomanToInt(s string) int {
	value := map[byte]int{
		'I': 1,
		'V': 5,
		'X': 10,
		'L': 50,
		'C': 100,
		'D': 500,
		'M': 1000,
	}
	total := 0
	for i := 0; i < len(s); i++ {
		v := value[s[i]]
		if i+1 < len(s) && v < value[s[i+1]] {
			total -= v
		} else {
			total += v
		}
	}
	return total
}
