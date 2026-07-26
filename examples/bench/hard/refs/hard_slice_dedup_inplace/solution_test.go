package solution

import "testing"

func TestVBasic(t *testing.T) {
	s := []int{1, 1, 2, 3, 3, 3, 4}
	k := DedupSortedInPlace(s)
	if k != 4 {
		t.Fatalf("k = %d, want 4", k)
	}
	got := s[:k]
	want := []int{1, 2, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("s[:k] = %v, want %v", got, want)
		}
	}
}

func TestVEmpty(t *testing.T) {
	if k := DedupSortedInPlace([]int{}); k != 0 {
		t.Fatalf("empty: k = %d, want 0", k)
	}
	if k := DedupSortedInPlace(nil); k != 0 {
		t.Fatalf("nil: k = %d, want 0", k)
	}
}

func TestVNoDuplicates(t *testing.T) {
	s := []int{1, 2, 3, 4, 5}
	k := DedupSortedInPlace(s)
	if k != 5 {
		t.Fatalf("k = %d, want 5", k)
	}
	for i := range s {
		if s[i] != i+1 {
			t.Fatalf("s = %v, want [1..5]", s)
		}
	}
}

func TestVAllSame(t *testing.T) {
	s := []int{7, 7, 7, 7}
	k := DedupSortedInPlace(s)
	if k != 1 || s[0] != 7 {
		t.Fatalf("k = %d, s[0] = %d, want k=1 s[0]=7", k, s[0])
	}
}

// TestH is the trap: the dedup must be truly in place. A naive solution that
// allocates a fresh result slice and returns its length will pass the visible
// tests but fail here, because the distinct values must be written back into
// the ORIGINAL backing array. We verify this by aliasing the backing array
// through a second slice header and checking the distinct prefix appears there.
func TestHInPlaceBackingArray(t *testing.T) {
	s := []int{1, 1, 2, 2, 5, 5, 5, 9}
	alias := s[:] // shares the same backing array as s

	k := DedupSortedInPlace(s)
	if k != 4 {
		t.Fatalf("k = %d, want 4", k)
	}

	want := []int{1, 2, 5, 9}
	// The distinct values must be visible through the aliased backing array,
	// proving the writes happened in place rather than into a new allocation.
	for i := range want {
		if alias[i] != want[i] {
			t.Fatalf("aliased backing array = %v, want prefix %v (writes were not in place)", alias[:k], want)
		}
	}

	// Reslicing the original slice to k must yield the distinct values.
	prefix := s[:k]
	for i := range want {
		if prefix[i] != want[i] {
			t.Fatalf("s[:k] = %v, want %v", prefix, want)
		}
	}
}

// TestHNoOverAllocation confirms the returned k never exceeds len(s) and the
// prefix is monotonically increasing (a correctness property of dedup on a
// sorted input).
func TestHMonotonePrefix(t *testing.T) {
	s := []int{-3, -3, 0, 0, 0, 1, 2, 2, 100}
	k := DedupSortedInPlace(s)
	if k > len(s) {
		t.Fatalf("k = %d exceeds len(s) = %d", k, len(s))
	}
	for i := 1; i < k; i++ {
		if s[i] <= s[i-1] {
			t.Fatalf("prefix not strictly increasing at %d: %v", i, s[:k])
		}
	}
	if k != 5 {
		t.Fatalf("k = %d, want 5", k)
	}
}
