// Package solution computes an integer histogram with bounded concurrency.
package solution

import "sync"

// ParallelHistogram counts how many times each value appears in nums, sharding
// the work across up to workers goroutines and merging their partial counts
// into a single map.
//
// If workers <= 0, one worker per element is used. An empty input yields an
// empty (non-nil) map. Each goroutine tallies into its own local map and the
// partials are merged under a mutex, so shared state is data-race-free and the
// result equals a sequential tally.
func ParallelHistogram(nums []int, workers int) map[int]int {
	result := make(map[int]int)
	n := len(nums)
	if n == 0 {
		return result
	}
	if workers <= 0 || workers > n {
		workers = n
	}

	chunk := (n + workers - 1) / workers
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := range workers {
		start := w * chunk
		if start >= n {
			break
		}
		end := min(start+chunk, n)
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			local := make(map[int]int)
			for i := start; i < end; i++ {
				local[nums[i]]++
			}
			mu.Lock()
			for k, v := range local {
				result[k] += v
			}
			mu.Unlock()
		}(start, end)
	}
	wg.Wait()
	return result
}
