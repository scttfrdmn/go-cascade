package solution

import "testing"

func TestVBasic(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"hello", "hello"},
		{"hello world", "world hello"},
		{"a b c d", "d c b a"},
		{"the quick brown fox", "fox brown quick the"},
	}
	for _, c := range cases {
		if got := ReverseWords(c.in); got != c.want {
			t.Fatalf("ReverseWords(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVTwoWords(t *testing.T) {
	if got := ReverseWords("one two"); got != "two one" {
		t.Fatalf("got %q, want %q", got, "two one")
	}
}

func TestHEmpty(t *testing.T) {
	if got := ReverseWords(""); got != "" {
		t.Fatalf("empty: got %q, want %q", got, "")
	}
}

func TestHSingleWordNoSpaces(t *testing.T) {
	if got := ReverseWords("word"); got != "word" {
		t.Fatalf("single: got %q, want %q", got, "word")
	}
}

func TestHRoundTrip(t *testing.T) {
	s := "alpha beta gamma delta epsilon"
	if got := ReverseWords(ReverseWords(s)); got != s {
		t.Fatalf("round trip: got %q, want %q", got, s)
	}
}

func TestHUnicode(t *testing.T) {
	if got := ReverseWords("café naïve über"); got != "über naïve café" {
		t.Fatalf("unicode: got %q", got)
	}
}

func TestHNoTrailingSpace(t *testing.T) {
	got := ReverseWords("x y z")
	if len(got) == 0 || got[0] == ' ' || got[len(got)-1] == ' ' {
		t.Fatalf("unexpected leading/trailing space: %q", got)
	}
}
