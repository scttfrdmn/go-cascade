// Package solution implements a two-stage concurrent pipeline.
package solution

// Pipeline applies stage1 and then stage2 to each element of in, running the
// two stages as concurrent pipeline steps connected by a channel, and returns
// the results in input order.
//
// stage1 runs in its own goroutine, feeding each intermediate value (tagged
// with its original index) to stage2 over a channel. stage2 runs in the
// calling goroutine and writes into a preallocated result slice by index, so
// order is preserved and there is no shared mutable state between the two
// stages. An empty input yields an empty (non-nil) result.
func Pipeline[T, M, R any](in []T, stage1 func(T) M, stage2 func(M) R) []R {
	out := make([]R, len(in))
	if len(in) == 0 {
		return out
	}

	type item struct {
		idx int
		val M
	}
	ch := make(chan item)
	go func() {
		defer close(ch)
		for i, v := range in {
			ch <- item{idx: i, val: stage1(v)}
		}
	}()
	for it := range ch {
		out[it.idx] = stage2(it.val)
	}
	return out
}
