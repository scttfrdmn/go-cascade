package solution

import "testing"

func TestVMostFrequentClearWinner(t *testing.T) {
	got, ok := MostFrequent([]int{1, 2, 2, 3, 2})
	if !ok || got != 2 {
		t.Errorf("got (%d, %v), want (2, true)", got, ok)
	}
}

func TestVMostFrequentSingle(t *testing.T) {
	got, ok := MostFrequent([]int{7})
	if !ok || got != 7 {
		t.Errorf("got (%d, %v), want (7, true)", got, ok)
	}
}

func TestVMostFrequentTieSmallest(t *testing.T) {
	got, ok := MostFrequent([]int{5, 3, 5, 3})
	if !ok || got != 3 {
		t.Errorf("got (%d, %v), want (3, true)", got, ok)
	}
}

func TestHMostFrequentEmpty(t *testing.T) {
	if got, ok := MostFrequent(nil); ok || got != 0 {
		t.Errorf("got (%d, %v), want (0, false)", got, ok)
	}
	if got, ok := MostFrequent([]int{}); ok || got != 0 {
		t.Errorf("got (%d, %v), want (0, false)", got, ok)
	}
}

func TestHMostFrequentAllTieNegatives(t *testing.T) {
	got, ok := MostFrequent([]int{3, -1, -5, 2, 0})
	if !ok || got != -5 {
		t.Errorf("got (%d, %v), want (-5, true)", got, ok)
	}
}

func TestHMostFrequentDeterministicTie(t *testing.T) {
	in := []int{10, 10, 20, 20, 5, 5}
	for i := 0; i < 100; i++ {
		got, ok := MostFrequent(in)
		if !ok || got != 5 {
			t.Fatalf("iter %d: got (%d, %v), want (5, true)", i, got, ok)
		}
	}
}

func TestHMostFrequentLarge(t *testing.T) {
	in := make([]int, 0, 3000)
	for i := 0; i < 1000; i++ {
		in = append(in, 1, 2, 2)
	}
	got, ok := MostFrequent(in)
	if !ok || got != 2 {
		t.Errorf("got (%d, %v), want (2, true)", got, ok)
	}
}
