package solution

// Average returns the arithmetic mean of the non-empty slice nums. The caller
// must provide at least one element; the result for a single element is that
// element itself.
func Average(nums []float64) float64 {
	var sum float64
	for _, v := range nums {
		sum += v
	}
	return sum / float64(len(nums))
}
