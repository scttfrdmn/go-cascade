package solution

import "testing"

func TestVRomanBasic(t *testing.T) {
	cases := map[string]int{
		"I": 1, "III": 3, "IV": 4, "V": 5, "IX": 9, "X": 10,
		"XL": 40, "L": 50, "XC": 90, "C": 100, "CD": 400,
		"D": 500, "CM": 900, "M": 1000,
	}
	for s, want := range cases {
		if got := RomanToInt(s); got != want {
			t.Errorf("RomanToInt(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestVRomanCompound(t *testing.T) {
	cases := map[string]int{
		"LVIII": 58, "MCMXCIV": 1994, "XIV": 14, "XXVII": 27, "DCXXI": 621,
	}
	for s, want := range cases {
		if got := RomanToInt(s); got != want {
			t.Errorf("RomanToInt(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestHRomanEmpty(t *testing.T) {
	if got := RomanToInt(""); got != 0 {
		t.Errorf(`RomanToInt("") = %d, want 0`, got)
	}
}

func TestHRomanMax(t *testing.T) {
	if got := RomanToInt("MMMDCCCLXXXVIII"); got != 3888 {
		t.Errorf("got %d, want 3888", got)
	}
	if got := RomanToInt("MMMCMXCIX"); got != 3999 {
		t.Errorf("got %d, want 3999", got)
	}
}

func TestHRomanAllSubtractive(t *testing.T) {
	cases := map[string]int{
		"IV": 4, "IX": 9, "XL": 40, "XC": 90, "CD": 400, "CM": 900,
	}
	for s, want := range cases {
		if got := RomanToInt(s); got != want {
			t.Errorf("RomanToInt(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestHRomanRoundTrip(t *testing.T) {
	symbols := []struct {
		val int
		sym string
	}{
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	}
	encode := func(n int) string {
		out := ""
		for _, sv := range symbols {
			for n >= sv.val {
				out += sv.sym
				n -= sv.val
			}
		}
		return out
	}
	for n := 1; n <= 3999; n++ {
		s := encode(n)
		if got := RomanToInt(s); got != n {
			t.Fatalf("RomanToInt(%q) = %d, want %d", s, got, n)
		}
	}
}
