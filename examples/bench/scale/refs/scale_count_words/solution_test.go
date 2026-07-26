package solution

import "testing"

func TestVBasic(t *testing.T) {
	if got := CountWords("hello world"); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}

func TestVSingle(t *testing.T) {
	if got := CountWords("hello"); got != 1 {
		t.Fatalf("got %d want 1", got)
	}
}

func TestVThreeWords(t *testing.T) {
	if got := CountWords("a b c"); got != 3 {
		t.Fatalf("got %d want 3", got)
	}
}

func TestHEmpty(t *testing.T) {
	if got := CountWords(""); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestHAllSpaces(t *testing.T) {
	if got := CountWords("     "); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestHLeadingTrailingRepeated(t *testing.T) {
	if got := CountWords("   foo   bar   "); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}

func TestHNonSpaceWhitespaceIsWord(t *testing.T) {
	// Tabs and newlines are NOT separators; only ASCII space is.
	if got := CountWords("a\tb\nc"); got != 1 {
		t.Fatalf("got %d want 1", got)
	}
}

func TestHTabsWithinCountAsOneWord(t *testing.T) {
	if got := CountWords("foo\tbar baz"); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}

func TestHSingleSpace(t *testing.T) {
	if got := CountWords(" "); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}
