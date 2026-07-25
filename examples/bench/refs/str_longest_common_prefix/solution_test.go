package solution

import "testing"

func TestVBasic(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"flower", "flow", "flight"}, "fl"},
		{[]string{"dog", "racecar", "car"}, ""},
		{[]string{"interspecies", "interstellar", "interstate"}, "inters"},
		{[]string{"prefix", "prefix", "prefix"}, "prefix"},
		{[]string{"single"}, "single"},
	}
	for _, c := range cases {
		if got := LongestCommonPrefix(c.in); got != c.want {
			t.Errorf("LongestCommonPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVFullMatch(t *testing.T) {
	if got := LongestCommonPrefix([]string{"abc", "abcd", "ab"}); got != "ab" {
		t.Errorf("got %q, want %q", got, "ab")
	}
}

func TestHEmptySlice(t *testing.T) {
	if got := LongestCommonPrefix(nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
	if got := LongestCommonPrefix([]string{}); got != "" {
		t.Errorf("empty slice: got %q, want empty", got)
	}
}

func TestHEmptyStringMember(t *testing.T) {
	// An empty string forces the common prefix to empty.
	if got := LongestCommonPrefix([]string{"abc", "", "abd"}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got := LongestCommonPrefix([]string{"", ""}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestHFirstShortest(t *testing.T) {
	// The shortest string is first; prefix cannot exceed it.
	if got := LongestCommonPrefix([]string{"a", "ab", "abc"}); got != "a" {
		t.Errorf("got %q, want %q", got, "a")
	}
}

func TestHNoCommon(t *testing.T) {
	if got := LongestCommonPrefix([]string{"x", "y"}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestHMultibyteAndBinary(t *testing.T) {
	// Byte-wise comparison over UTF-8: shared leading rune.
	if got := LongestCommonPrefix([]string{"café", "cafételia"}); got != "café" {
		t.Errorf("got %q, want %q", got, "café")
	}
	// Bytes with NUL and high bytes.
	a := "\x00\xff\x10rest"
	b := "\x00\xff\x10other"
	if got := LongestCommonPrefix([]string{a, b}); got != "\x00\xff\x10" {
		t.Errorf("got %q, want %q", got, "\x00\xff\x10")
	}
}

func TestHIdenticalLong(t *testing.T) {
	long := ""
	for i := 0; i < 5000; i++ {
		long += "z"
	}
	if got := LongestCommonPrefix([]string{long, long, long}); got != long {
		t.Errorf("long identical strings failed: len(got)=%d", len(got))
	}
}
