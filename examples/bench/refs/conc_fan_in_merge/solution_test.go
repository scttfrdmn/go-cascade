package solution

import (
	"sort"
	"testing"
)

func feed(vals ...int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for _, v := range vals {
			ch <- v
		}
	}()
	return ch
}

func isAscending(s []int) bool {
	for i := 1; i < len(s); i++ {
		if s[i] < s[i-1] {
			return false
		}
	}
	return true
}

func TestVMergeThree(t *testing.T) {
	got := MergeSorted(feed(1, 4, 7), feed(2, 5, 8), feed(3, 6, 9))
	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestVNoChannels(t *testing.T) {
	got := MergeSorted()
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestVSingleChannel(t *testing.T) {
	got := MergeSorted(feed(1, 2, 3))
	if len(got) != 3 || !isAscending(got) {
		t.Fatalf("got %v", got)
	}
}

func TestHEmptyChannelsMixed(t *testing.T) {
	got := MergeSorted(feed(), feed(5), feed(), feed(1, 2))
	want := []int{1, 2, 5}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestHManyChannelsLarge(t *testing.T) {
	const nch, per = 50, 400
	chans := make([]<-chan int, nch)
	want := make([]int, 0, nch*per)
	for c := range nch {
		vals := make([]int, per)
		for i := range per {
			// ascending within each channel; overlapping ranges across channels
			vals[i] = i*nch + c
			want = append(want, vals[i])
		}
		chans[c] = feed(vals...)
	}
	got := MergeSorted(chans...)
	if len(got) != nch*per {
		t.Fatalf("got %d values want %d", len(got), nch*per)
	}
	if !isAscending(got) {
		t.Fatal("result not ascending")
	}
	sort.Ints(want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestHDuplicates(t *testing.T) {
	got := MergeSorted(feed(1, 1, 2), feed(1, 2, 2))
	want := []int{1, 1, 1, 2, 2, 2}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}
