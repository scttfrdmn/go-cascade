package solution

import "testing"

func seqHist(nums []int) map[int]int {
	m := make(map[int]int)
	for _, v := range nums {
		m[v]++
	}
	return m
}

func equalMaps(a, b map[int]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestVBasicCount(t *testing.T) {
	nums := []int{1, 2, 2, 3, 3, 3}
	got := ParallelHistogram(nums, 2)
	want := map[int]int{1: 1, 2: 2, 3: 3}
	if !equalMaps(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestVEmpty(t *testing.T) {
	got := ParallelHistogram(nil, 4)
	if got == nil {
		t.Fatal("expected non-nil map")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestVNegativesAndZero(t *testing.T) {
	nums := []int{-1, 0, -1, 0, 0}
	got := ParallelHistogram(nums, 3)
	want := map[int]int{-1: 2, 0: 3}
	if !equalMaps(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestHZeroAndNegativeWorkers(t *testing.T) {
	nums := []int{5, 5, 6, 7, 7, 7}
	want := seqHist(nums)
	for _, w := range []int{0, -1, -30} {
		got := ParallelHistogram(nums, w)
		if !equalMaps(got, want) {
			t.Fatalf("workers=%d got %v want %v", w, got, want)
		}
	}
}

func TestHLargeManyWorkers(t *testing.T) {
	const n = 30000
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i % 97
	}
	want := seqHist(nums)
	for _, w := range []int{1, 3, 8, 64, 256, n, n + 10} {
		got := ParallelHistogram(nums, w)
		if !equalMaps(got, want) {
			t.Fatalf("workers=%d histogram mismatch", w)
		}
	}
}

func TestHTotalCountPreserved(t *testing.T) {
	const n = 12345
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i % 5
	}
	got := ParallelHistogram(nums, 50)
	total := 0
	for _, v := range got {
		total += v
	}
	if total != n {
		t.Fatalf("total count %d want %d", total, n)
	}
}
