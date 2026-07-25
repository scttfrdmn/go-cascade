package solution

import "testing"

func TestVReverse(t *testing.T) {
	got := ReversedCopy([]int{1, 2, 3, 4})
	want := []int{4, 3, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestVEmptyAndSingle(t *testing.T) {
	if got := ReversedCopy([]int{}); len(got) != 0 {
		t.Fatalf("empty: got %v, want []", got)
	}
	got := ReversedCopy([]int{42})
	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("single: got %v, want [42]", got)
	}
}

// TestH is the trap: the function must NOT mutate the caller's input, including
// the backing array. An in-place reversal that returns the same slice would
// pass a naive equality check on the result but corrupt the caller's data.
// We snapshot the input, call, and assert the input is byte-for-byte unchanged.
func TestHNoMutation(t *testing.T) {
	in := []int{5, 6, 7, 8, 9}
	snapshot := make([]int, len(in))
	copy(snapshot, in)

	got := ReversedCopy(in)

	// Input must be unchanged.
	for i := range snapshot {
		if in[i] != snapshot[i] {
			t.Fatalf("input mutated: in = %v, want %v", in, snapshot)
		}
	}

	// Result must actually be reversed.
	for i := range in {
		if got[i] != in[len(in)-1-i] {
			t.Fatalf("result not reversed: got %v, in %v", got, in)
		}
	}
}

// TestHIndependentBackingArray verifies the result does not alias the input's
// backing array: mutating the returned slice must not change the input, and
// vice versa.
func TestHIndependentBackingArray(t *testing.T) {
	in := []int{1, 2, 3}
	got := ReversedCopy(in)

	got[0] = 999
	if in[2] == 999 {
		t.Fatalf("result aliases input backing array: mutating result changed input %v", in)
	}

	in[0] = -1
	if got[len(got)-1] == -1 {
		t.Fatalf("input aliases result backing array: mutating input changed result %v", got)
	}
}

// TestHAliasedInputSubslice guards against a subtle backing-array bug: if the
// input is a subslice of a larger array, an in-place reversal would corrupt the
// surrounding elements. Here we assert those neighbours survive untouched.
func TestHSurroundingArrayIntact(t *testing.T) {
	backing := []int{100, 1, 2, 3, 200}
	in := backing[1:4] // {1, 2, 3}, sharing backing

	_ = ReversedCopy(in)

	if backing[0] != 100 || backing[4] != 200 {
		t.Fatalf("surrounding array corrupted: %v", backing)
	}
	if backing[1] != 1 || backing[2] != 2 || backing[3] != 3 {
		t.Fatalf("input region mutated via backing array: %v", backing)
	}
}
