package solution

// IntegerMean returns the floor of the arithmetic mean of a non-empty slice of
// int64 values. It is correct even when the exact sum of the elements would
// overflow int64.
//
// The naive approach (summing every element into an int64 and dividing by the
// count) silently overflows for large inputs. Instead we maintain a running
// quotient and remainder so that no intermediate value ever exceeds the range
// of the operands.
//
// "Floor" here means the result is rounded toward negative infinity, matching
// mathematical floor division. For example the mean of {-1, -2} is -1.5, and
// IntegerMean returns -2.
//
// IntegerMean panics if given an empty slice, since the mean is undefined.
func IntegerMean(xs []int64) int64 {
	if len(xs) == 0 {
		panic("IntegerMean: empty slice")
	}

	n := int64(len(xs))

	// Accumulate the mean incrementally. We track:
	//   quo - the running floored quotient (the mean so far)
	//   rem - the running remainder in [0, n)
	// After processing all elements, quo*n + rem == sum(xs) exactly, with
	// 0 <= rem < n, so quo is exactly floor(sum/n).
	//
	// For each element x we add x to the conceptual total. We split x itself
	// into its quotient and remainder against n (using floored division so the
	// invariant 0 <= rem < n is preserved for negative values), add those, then
	// carry any overflow of rem back into quo. No intermediate value exceeds the
	// magnitude of x plus n, so int64 never overflows.
	var quo, rem int64
	for _, x := range xs {
		q := x / n
		r := x % n
		// Go's % can be negative; normalize to a floored remainder in [0, n).
		if r < 0 {
			r += n
			q--
		}
		quo += q
		rem += r
		if rem >= n {
			rem -= n
			quo++
		}
	}

	return quo
}
