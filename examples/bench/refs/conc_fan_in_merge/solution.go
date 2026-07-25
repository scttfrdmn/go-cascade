// Package solution merges several sorted integer channels into one sorted slice.
package solution

import (
	"sort"
	"sync"
)

// MergeSorted drains every channel in chans concurrently and returns a single
// ascending slice containing all values from all channels.
//
// Each input channel is expected to yield integers in ascending order and
// then close. Draining happens concurrently: one goroutine per channel feeds
// a shared buffered collector, and the combined result is sorted before
// return, so the output is globally ascending regardless of interleaving.
// No inputs yields an empty (non-nil) slice.
func MergeSorted(chans ...<-chan int) []int {
	out := make([]int, 0)
	if len(chans) == 0 {
		return out
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ch := range chans {
		wg.Add(1)
		go func(ch <-chan int) {
			defer wg.Done()
			local := make([]int, 0)
			for v := range ch {
				local = append(local, v)
			}
			mu.Lock()
			out = append(out, local...)
			mu.Unlock()
		}(ch)
	}
	wg.Wait()

	sort.Ints(out)
	return out
}
