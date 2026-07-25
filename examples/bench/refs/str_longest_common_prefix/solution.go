package solution

// LongestCommonPrefix returns the longest common prefix shared by all
// strings in strs. If strs is empty or the strings share no common
// prefix, it returns the empty string. Comparison is byte-wise.
func LongestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		// Shrink prefix until it is a prefix of s.
		n := len(prefix)
		if len(s) < n {
			n = len(s)
		}
		i := 0
		for i < n && prefix[i] == s[i] {
			i++
		}
		prefix = prefix[:i]
		if prefix == "" {
			return ""
		}
	}
	return prefix
}
