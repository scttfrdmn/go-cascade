package solution

import "testing"

func TestVBalancedSimple(t *testing.T) {
	good := []string{"()", "[]", "{}", "()[]{}", "([{}])", "{[()()]}"}
	for _, s := range good {
		if !IsBalanced(s) {
			t.Errorf("IsBalanced(%q) = false, want true", s)
		}
	}
}

func TestVBalancedUnbalanced(t *testing.T) {
	bad := []string{"(", ")", "([)]", "(]", "{[}", "((("}
	for _, s := range bad {
		if IsBalanced(s) {
			t.Errorf("IsBalanced(%q) = true, want false", s)
		}
	}
}

func TestVBalancedEmpty(t *testing.T) {
	if !IsBalanced("") {
		t.Error("empty string should be balanced")
	}
}

func TestHBalancedInvalidChars(t *testing.T) {
	bad := []string{"(a)", "hello", "1+2", "( )", "[]{}x"}
	for _, s := range bad {
		if IsBalanced(s) {
			t.Errorf("IsBalanced(%q) = true, want false (invalid char)", s)
		}
	}
}

func TestHBalancedWrongClose(t *testing.T) {
	if IsBalanced("[(])") {
		t.Error("interleaved brackets should be false")
	}
	if IsBalanced("{[()]}}") {
		t.Error("extra close should be false")
	}
	if IsBalanced("{{[()]}") {
		t.Error("extra open should be false")
	}
}

func TestHBalancedCloseBeforeOpen(t *testing.T) {
	if IsBalanced(")(") {
		t.Error("close before open should be false")
	}
}

func TestHBalancedDeepNesting(t *testing.T) {
	const depth = 5000
	b := make([]byte, 0, depth*2)
	for i := 0; i < depth; i++ {
		b = append(b, '(')
	}
	for i := 0; i < depth; i++ {
		b = append(b, ')')
	}
	if !IsBalanced(string(b)) {
		t.Error("deep balanced nesting should be true")
	}
}
