package solution

import "testing"

func TestVBasicTotal(t *testing.T) {
	if got := ConcurrentCount(4, 100); got != 400 {
		t.Fatalf("got %d want 400", got)
	}
}

func TestVSingleGoroutine(t *testing.T) {
	if got := ConcurrentCount(1, 50); got != 50 {
		t.Fatalf("got %d want 50", got)
	}
}

func TestHZeroInputs(t *testing.T) {
	cases := [][2]int{{0, 10}, {10, 0}, {0, 0}, {-1, 5}, {5, -1}}
	for _, c := range cases {
		if got := ConcurrentCount(c[0], c[1]); got != 0 {
			t.Fatalf("ConcurrentCount(%d,%d)=%d want 0", c[0], c[1], got)
		}
	}
}

func TestHExactlyOnceHighContention(t *testing.T) {
	const g, inc = 128, 5000
	got := ConcurrentCount(g, inc)
	want := int64(g) * int64(inc)
	if got != want {
		t.Fatalf("got %d want %d (lost or double-counted increments)", got, want)
	}
}

func TestHRepeatable(t *testing.T) {
	for range 5 {
		if got := ConcurrentCount(32, 1000); got != 32000 {
			t.Fatalf("got %d want 32000", got)
		}
	}
}
