// Package solution implements a lazy, concurrency-safe memoizing cache in
// which the compute function for any given key runs exactly once, even when
// many goroutines request the same key at the same time.
package solution

import "sync"

// entry holds the memoized value for a single key together with a sync.Once
// that guarantees the value is computed exactly once.
type entry struct {
	once  sync.Once
	value any
}

// LazyCache maps string keys to lazily computed values. The first Get for a
// key runs the supplied compute function; concurrent and subsequent Gets for
// the same key block until that single computation completes and then observe
// the identical value. compute is never invoked more than once per key.
//
// The zero value is not ready for use; call NewLazyCache.
type LazyCache struct {
	mu      sync.Mutex
	entries map[string]*entry
}

// NewLazyCache returns an empty, ready-to-use LazyCache.
func NewLazyCache() *LazyCache {
	return &LazyCache{entries: make(map[string]*entry)}
}

// Get returns the value associated with key, computing it via compute on the
// first call for that key. All callers for the same key observe the same
// value, and compute runs exactly once per key regardless of concurrency.
//
// The outer lock is held only long enough to find-or-create the per-key entry;
// the potentially slow compute call runs outside that lock (inside the entry's
// sync.Once), so distinct keys never serialize against each other.
func (c *LazyCache) Get(key string, compute func() any) any {
	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		e = &entry{}
		c.entries[key] = e
	}
	c.mu.Unlock()

	e.once.Do(func() {
		e.value = compute()
	})
	return e.value
}
