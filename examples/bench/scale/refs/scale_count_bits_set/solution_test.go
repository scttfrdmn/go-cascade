package solution

import (
	"math/bits"
	"testing"
)

func TestVSmall(t *testing.T) {
	got := CountBits(5)
	want := []int{0, 1, 1, 2, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("CountBits(5)[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestVTwo(t *testing.T) {
	got := CountBits(2)
	want := []int{0, 1, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("CountBits(2)[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestHZero(t *testing.T) {
	got := CountBits(0)
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("CountBits(0) = %v, want [0]", got)
	}
}

func TestHNegative(t *testing.T) {
	got := CountBits(-1)
	if len(got) != 0 {
		t.Errorf("CountBits(-1) = %v, want empty", got)
	}
}

func TestHLargeAgainstStdlib(t *testing.T) {
	n := 100000
	got := CountBits(n)
	if len(got) != n+1 {
		t.Fatalf("len = %d, want %d", len(got), n+1)
	}
	for i := 0; i <= n; i++ {
		want := bits.OnesCount(uint(i))
		if got[i] != want {
			t.Fatalf("CountBits(%d)[%d] = %d, want %d", n, i, got[i], want)
		}
	}
}
