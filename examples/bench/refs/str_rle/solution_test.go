package solution

import (
	"strings"
	"testing"
)

func TestVBasic(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"a", "a1"},
		{"aaabbc", "a3b2c1"},
		{"aaaa", "a4"},
		{"abc", "a1b1c1"},
		{"aabbaa", "a2b2a2"},
	}
	for _, c := range cases {
		if got := RunLengthEncode(c.in); got != c.want {
			t.Fatalf("RunLengthEncode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVSingleChar(t *testing.T) {
	if got := RunLengthEncode("z"); got != "z1" {
		t.Fatalf("got %q, want z1", got)
	}
}

func TestHEmpty(t *testing.T) {
	if got := RunLengthEncode(""); got != "" {
		t.Fatalf("empty: got %q, want empty", got)
	}
}

func TestHLongRunCountMultiDigit(t *testing.T) {
	in := strings.Repeat("x", 123)
	if got := RunLengthEncode(in); got != "x123" {
		t.Fatalf("got %q, want x123", got)
	}
}

func TestHAlternating(t *testing.T) {
	in := "ababab"
	want := "a1b1a1b1a1b1"
	if got := RunLengthEncode(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHDigitsInInput(t *testing.T) {
	// The input bytes may themselves be digits; encoding is still by byte.
	if got := RunLengthEncode("111"); got != "13" {
		t.Fatalf("got %q, want 13", got)
	}
	if got := RunLengthEncode("122"); got != "1122" {
		t.Fatalf("got %q, want 1122", got)
	}
}

func TestHNonASCIIBytes(t *testing.T) {
	// "é" in UTF-8 is two bytes 0xC3 0xA9; "éé" -> those two bytes twice.
	got := RunLengthEncode("éé")
	// bytes: C3 A9 C3 A9 -> runs of length 1 each
	want := string([]byte{0xC3}) + "1" + string([]byte{0xA9}) + "1" +
		string([]byte{0xC3}) + "1" + string([]byte{0xA9}) + "1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHLarge(t *testing.T) {
	in := strings.Repeat("a", 100000)
	if got := RunLengthEncode(in); got != "a100000" {
		t.Fatalf("large: got %q", got)
	}
}
