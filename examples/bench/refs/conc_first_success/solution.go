// Package solution runs functions concurrently and returns the first success.
package solution

import (
	"context"
	"errors"
	"sync"
)

// ErrNoFuncs is returned when FirstSuccess is called with no functions.
var ErrNoFuncs = errors.New("first_success: no functions provided")

// FirstSuccess runs every function in fns concurrently and returns the result
// of the first one to complete with a nil error. As soon as a success is
// observed, the context passed to the remaining functions is cancelled so they
// can stop early.
//
// The provided ctx is used as the parent; each function receives a derived
// context that is cancelled on the first success or when all have finished.
// If every function fails, FirstSuccess returns the last error observed. If
// fns is empty, it returns (0, ErrNoFuncs).
func FirstSuccess(ctx context.Context, fns []func(context.Context) (int, error)) (int, error) {
	if len(fns) == 0 {
		return 0, ErrNoFuncs
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type outcome struct {
		val int
		err error
	}
	results := make(chan outcome, len(fns))
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Add(1)
		go func(fn func(context.Context) (int, error)) {
			defer wg.Done()
			v, err := fn(childCtx)
			results <- outcome{v, err}
		}(fn)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var lastErr error
	for r := range results {
		if r.err == nil {
			cancel() // stop the rest
			return r.val, nil
		}
		lastErr = r.err
	}
	return 0, lastErr
}
