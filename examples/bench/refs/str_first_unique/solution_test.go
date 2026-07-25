package solution

import "testing"

// bruteFirstUnique is an independent reference implementation.
func bruteFirstUnique(s string) int {
	for i := 0; i < len(s); i++ {
		seen := false
		for j := 0; j < len(s); j++ {
			if j != i && s[j] == s[i] {
				seen = true
				break
			}
		}
		if !seen {
			return i
		}
	}
	return -1
}

func TestVBasic(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"leetcode", 0},
		{"loveleetcode", 2},
		{"aabb", -1},
		{"abcabc", -1},
		{"z", 0},
		{"abcd", 0},
		{"aabbc", 4},
		{"swiss", 1}, // w is first unique (s repeats, w unique)
	}
	for _, c := range cases {
		if got := FirstUniqueByte(c.in); got != c.want {
			t.Errorf("FirstUniqueByte(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestVAllRepeat(t *testing.T) {
	if got := FirstUniqueByte("aabbcc"); got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

func TestHEmpty(t *testing.T) {
	if got := FirstUniqueByte(""); got != -1 {
		t.Errorf("empty: got %d, want -1", got)
	}
}

func TestHUniqueAtEnd(t *testing.T) {
	if got := FirstUniqueByte("aabbccd"); got != 6 {
		t.Errorf("got %d, want 6", got)
	}
}

func TestHBinaryBytes(t *testing.T) {
	// NUL and high bytes; the unique one is \x01 at index 2.
	s := "\x00\x00\x01\x00"
	if got := FirstUniqueByte(s); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
	// 0xff appears once at index 0.
	s2 := "\xff\x00\x00"
	if got := FirstUniqueByte(s2); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestHMultibyteRunes(t *testing.T) {
	// Operates on bytes, not runes. "é" is two bytes (0xc3 0xa9).
	// In "aé" -> 'a'(1) then 0xc3,0xa9 each unique; first unique byte is 'a' at 0.
	if got := FirstUniqueByte("aé"); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestHAgainstBrute(t *testing.T) {
	var seed uint64 = 99
	next := func(mod int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		v := int(seed>>33) % mod
		if v < 0 {
			v += mod
		}
		return v
	}
	for trial := 0; trial < 3000; trial++ {
		n := next(20)
		b := make([]byte, n)
		for i := range b {
			b[i] = byte('a' + next(4)) // small alphabet to force collisions
		}
		s := string(b)
		want := bruteFirstUnique(s)
		if got := FirstUniqueByte(s); got != want {
			t.Fatalf("FirstUniqueByte(%q) = %d, want %d", s, got, want)
		}
	}
}
