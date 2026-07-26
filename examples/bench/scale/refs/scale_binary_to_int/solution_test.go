package solution

import "testing"

func TestVBasic(t *testing.T) {
	cases := map[string]uint64{
		"0":    0,
		"1":    1,
		"10":   2,
		"101":  5,
		"1111": 15,
	}
	for in, want := range cases {
		got, ok := BinaryToInt(in)
		if !ok || got != want {
			t.Fatalf("BinaryToInt(%q)=(%d,%v) want %d", in, got, ok, want)
		}
	}
}

func TestVLeadingZeros(t *testing.T) {
	got, ok := BinaryToInt("00001010")
	if !ok || got != 10 {
		t.Fatalf("got (%d,%v)", got, ok)
	}
}

func TestHEmpty(t *testing.T) {
	got, ok := BinaryToInt("")
	if ok || got != 0 {
		t.Fatalf("empty should be invalid, got (%d,%v)", got, ok)
	}
}

func TestHInvalidChars(t *testing.T) {
	for _, in := range []string{"2", "1a0", " 10", "10 ", "1.0", "0b1"} {
		got, ok := BinaryToInt(in)
		if ok || got != 0 {
			t.Fatalf("BinaryToInt(%q) should be invalid, got (%d,%v)", in, got, ok)
		}
	}
}

func TestHMaxWidth(t *testing.T) {
	// 64 ones == max uint64.
	s := ""
	for i := 0; i < 64; i++ {
		s += "1"
	}
	got, ok := BinaryToInt(s)
	if !ok || got != ^uint64(0) {
		t.Fatalf("64 ones: got (%d,%v)", got, ok)
	}
}
