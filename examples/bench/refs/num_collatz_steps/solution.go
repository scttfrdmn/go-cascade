package solution

// CollatzSteps returns the number of Collatz steps required to reach 1
// from the positive integer n. Each step maps an even value m to m/2 and
// an odd value m to 3m+1. The count for n == 1 is 0. For n == 0 (not a
// positive integer) it returns -1.
func CollatzSteps(n uint64) int {
	if n == 0 {
		return -1
	}
	steps := 0
	for n != 1 {
		if n%2 == 0 {
			n /= 2
		} else {
			n = 3*n + 1
		}
		steps++
	}
	return steps
}
