// Package solution provides a bounded-concurrency parallel map.
package solution

import "sync"

// ParallelMap applies f to every element of in using at most workers
// goroutines and returns the results in the same order as the input.
//
// If workers <= 0, one worker is used per element. An empty input yields
// an empty (non-nil) result. Each output slot is written by exactly one
// goroutine, so the operation is data-race-free.
func ParallelMap[T, R any](in []T, workers int, f func(T) R) []R {
	results := make([]R, len(in))
	if len(in) == 0 {
		return results
	}
	if workers <= 0 || workers > len(in) {
		workers = len(in)
	}

	idx := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				results[i] = f(in[i])
			}
		}()
	}
	for i := range in {
		idx <- i
	}
	close(idx)
	wg.Wait()
	return results
}
