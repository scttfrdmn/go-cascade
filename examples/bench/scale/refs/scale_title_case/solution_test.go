package solution

import "testing"

func TestVBasic(t *testing.T) {
	if got := TitleCase("hello world"); got != "Hello World" {
		t.Errorf("TitleCase = %q, want %q", got, "Hello World")
	}
}

func TestVMixedCase(t *testing.T) {
	if got := TitleCase("hELLo WORLD"); got != "Hello World" {
		t.Errorf("TitleCase = %q, want %q", got, "Hello World")
	}
}

func TestHSingleWord(t *testing.T) {
	if got := TitleCase("go"); got != "Go" {
		t.Errorf("TitleCase = %q, want %q", got, "Go")
	}
}

func TestHEmpty(t *testing.T) {
	if got := TitleCase(""); got != "" {
		t.Errorf("TitleCase(\"\") = %q, want empty", got)
	}
}

func TestHNonLetters(t *testing.T) {
	// Words beginning with digits/punctuation stay unchanged at the
	// front; trailing letters are lower-cased.
	if got := TitleCase("123ABC 9foo"); got != "123abc 9foo" {
		t.Errorf("TitleCase = %q, want %q", got, "123abc 9foo")
	}
}

func TestHManyWords(t *testing.T) {
	in := "the QUICK brown FOX jumps"
	want := "The Quick Brown Fox Jumps"
	if got := TitleCase(in); got != want {
		t.Errorf("TitleCase = %q, want %q", got, want)
	}
}

func TestHSingleLetters(t *testing.T) {
	if got := TitleCase("a b c"); got != "A B C" {
		t.Errorf("TitleCase = %q, want %q", got, "A B C")
	}
}
