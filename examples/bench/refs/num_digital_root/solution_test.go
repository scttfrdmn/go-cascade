package solution

import "testing"

func TestVDigitalRootBasic(t *testing.T) {
	cases := map[uint64]int{
		0:  0,
		5:  5,
		9:  9,
		10: 1,
		38: 2, // 3+8=11 -> 1+1=2
		99: 9, // 9+9=18 -> 1+8=9
		12: 3,
		18: 9,
	}
	for in, want := range cases {
		if got := DigitalRoot(in); got != want {
			t.Errorf("DigitalRoot(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestVDigitalRootSingleDigits(t *testing.T) {
	for i := uint64(0); i <= 9; i++ {
		if got := DigitalRoot(i); got != int(i) {
			t.Errorf("DigitalRoot(%d) = %d, want %d", i, got, i)
		}
	}
}

func TestHDigitalRootMultipleOfNine(t *testing.T) {
	for _, n := range []uint64{9, 18, 27, 81, 900, 999999999} {
		if got := DigitalRoot(n); got != 9 {
			t.Errorf("DigitalRoot(%d) = %d, want 9", n, got)
		}
	}
}

func TestHDigitalRootCongruence(t *testing.T) {
	for n := uint64(0); n <= 5000; n++ {
		want := 0
		if n > 0 {
			want = int(1 + (n-1)%9)
		}
		if got := DigitalRoot(n); got != want {
			t.Fatalf("DigitalRoot(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestHDigitalRootMaxUint64(t *testing.T) {
	const max = ^uint64(0)
	want := int(1 + (max-1)%9)
	if got := DigitalRoot(max); got != want {
		t.Errorf("DigitalRoot(max) = %d, want %d", got, want)
	}
}
