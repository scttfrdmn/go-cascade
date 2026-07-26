package solution

// Pair is a single key/value input record to be grouped.
type Pair struct {
	Key   string
	Value int
}

// GroupStable groups the values of pairs by key, returning a map from each key
// to the slice of its values in first-seen input order. The input is iterated
// exactly once, so within any key the values appear in the same relative order
// as they occur in pairs. A nil or empty input yields an empty (non-nil) map.
func GroupStable(pairs []Pair) map[string][]int {
	out := make(map[string][]int)
	for _, p := range pairs {
		out[p.Key] = append(out[p.Key], p.Value)
	}
	return out
}
