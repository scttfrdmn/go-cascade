// Package solution provides a bounded-concurrency parallel filter.
package solution

import "sync"

// ParallelFilter returns the elements of in for which keep returns true,
// preserving input order, while evaluating keep on up to workers goroutines
// concurrently.
//
// If workers <= 0, one worker per element is used. An empty input yields an
// empty (non-nil) result. Each keep decision is recorded in a dedicated
// slot, so evaluation is data-race-free; the surviving elements are then
// gathered sequentially in order.
func ParallelFilter[T any](in []T, workers int, keep func(T) bool) []T {
	out := make([]T, 0)
	if len(in) == 0 {
		return out
	}
	if workers <= 0 || workers > len(in) {
		workers = len(in)
	}

	verdict := make([]bool, len(in))
	idx := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				verdict[i] = keep(in[i])
			}
		}()
	}
	for i := range in {
		idx <- i
	}
	close(idx)
	wg.Wait()

	for i, v := range in {
		if verdict[i] {
			out = append(out, v)
		}
	}
	return out
}
