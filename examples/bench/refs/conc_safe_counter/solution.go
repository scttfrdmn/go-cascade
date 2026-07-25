// Package solution provides a race-free concurrent counter.
package solution

import (
	"sync"
	"sync/atomic"
)

// ConcurrentCount starts goroutines goroutines, each of which increments a
// single shared counter incrementsPerGoroutine times, waits for all of them
// to finish, and returns the final total.
//
// If goroutines <= 0 or incrementsPerGoroutine <= 0, the total is 0. The
// shared counter is accessed only through sync/atomic, so all increments are
// race-free and each is applied exactly once.
func ConcurrentCount(goroutines, incrementsPerGoroutine int) int64 {
	if goroutines <= 0 || incrementsPerGoroutine <= 0 {
		return 0
	}

	var counter atomic.Int64
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range incrementsPerGoroutine {
				counter.Add(1)
			}
		}()
	}
	wg.Wait()
	return counter.Load()
}
