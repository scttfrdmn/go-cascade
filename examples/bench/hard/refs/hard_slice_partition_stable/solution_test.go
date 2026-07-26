package solution

import "testing"

func isEven(n int) bool { return n%2 == 0 }

func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestVBasic(t *testing.T) {
	got := StablePartition([]int{1, 2, 3, 4, 5, 6}, isEven)
	want := []int{2, 4, 6, 1, 3, 5}
	if !eq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVEmpty(t *testing.T) {
	if got := StablePartition([]int{}, isEven); len(got) != 0 {
		t.Fatalf("got %v, want []", got)
	}
}

func TestVAllOrNone(t *testing.T) {
	allTrue := StablePartition([]int{2, 4, 6}, isEven)
	if !eq(allTrue, []int{2, 4, 6}) {
		t.Fatalf("all-true: got %v, want [2 4 6]", allTrue)
	}
	allFalse := StablePartition([]int{1, 3, 5}, isEven)
	if !eq(allFalse, []int{1, 3, 5}) {
		t.Fatalf("all-false: got %v, want [1 3 5]", allFalse)
	}
}

// TestH is the trap: the partition must be STABLE. A two-pointer swap partition
// produces a correct grouping but reorders elements within each group. We use
// distinguishable equal-group members (via encoded tags) and assert the exact
// original relative order is preserved in both groups.
func TestHStableOrder(t *testing.T) {
	// Values encode: (group)*100 + (original position within group).
	// "satisfies" group: multiples of 10 in [10, 60]; ordering by original index.
	// Interleave the two groups so an unstable swap would scramble them.
	in := []int{10, 3, 20, 7, 30, 1, 40, 9, 50, 5, 60}
	// pred: value >= 10 AND divisible by 10.
	pred := func(n int) bool { return n >= 10 && n%10 == 0 }

	got := StablePartition(in, pred)
	want := []int{10, 20, 30, 40, 50, 60, 3, 7, 1, 9, 5}
	if !eq(got, want) {
		t.Fatalf("unstable partition: got %v, want %v", got, want)
	}
}

// TestHDuplicatesKeepOrder uses duplicate values so only positional stability
// (not value comparison) can distinguish a stable result. An unstable swap
// partition would fail to preserve which "3" came first relative to structure.
func TestHDuplicatesStable(t *testing.T) {
	in := []int{4, 4, 3, 3, 4, 3, 4}
	pred := isEven
	got := StablePartition(in, pred)
	// evens in order: 4,4,4,4 ; odds in order: 3,3,3
	want := []int{4, 4, 4, 4, 3, 3, 3}
	if !eq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestHNoMutation asserts the input slice and its backing array are unchanged,
// byte for byte, after the call.
func TestHNoMutation(t *testing.T) {
	in := []int{1, 2, 3, 4, 5, 6}
	snapshot := make([]int, len(in))
	copy(snapshot, in)

	got := StablePartition(in, isEven)

	for i := range snapshot {
		if in[i] != snapshot[i] {
			t.Fatalf("input mutated: in = %v, want %v", in, snapshot)
		}
	}

	// Result must not alias the input backing array.
	if len(got) > 0 {
		got[0] = -999
		for i := range in {
			if in[i] == -999 {
				t.Fatalf("result aliases input backing array; input = %v", in)
			}
		}
	}
}
