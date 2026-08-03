package cluster

import "testing"

const canonical = `package solution

import "strings"

// Reverse returns s reversed.
func Reverse(s string) string {
	var b strings.Builder
	for i := len(s) - 1; i >= 0; i-- {
		b.WriteByte(s[i])
	}
	return b.String()
}
`

// The normalisation has to be generous enough that arm (e) is not a strawman: if
// comments and formatting split otherwise identical candidates, self-consistency
// loses on a technicality and §3.5's claim is untested. Each case below is a
// difference a model actually produces between two draws of the same answer.
func TestTextKeyIgnoresFormattingAndComments(t *testing.T) {
	same := map[string]string{
		"no comments": `package solution

import "strings"

func Reverse(s string) string {
	var b strings.Builder
	for i := len(s) - 1; i >= 0; i-- {
		b.WriteByte(s[i])
	}
	return b.String()
}
`,
		"different comments": `package solution

import "strings"

// Reverse flips a string end to end.
// Runs in O(n).
func Reverse(s string) string {
	var b strings.Builder
	for i := len(s) - 1; i >= 0; i-- {
		b.WriteByte(s[i])
	}
	return b.String()
}
`,
		"mangled indentation and blank lines": `package solution
import "strings"
func Reverse(s string) string {
        var b strings.Builder
        for i := len(s) - 1; i >= 0; i-- {
                        b.WriteByte(s[i])
        }

        return b.String()
}
`,
	}
	want := TextKey(canonical)
	for name, src := range same {
		if got := TextKey(src); got != want {
			t.Errorf("%s: key %s != canonical %s; a cosmetic difference split the vote, "+
				"which handicaps arm (e) for reasons unrelated to self-consistency", name, got, want)
		}
	}
}

// Import ORDER is cosmetic and models emit it inconsistently, so it must not split
// a vote either.
func TestTextKeyIgnoresImportOrder(t *testing.T) {
	a := `package solution

import (
	"sort"
	"strings"
)

func F(s []string) string { sort.Strings(s); return strings.Join(s, ",") }
`
	b := `package solution

import (
	"strings"
	"sort"
)

func F(s []string) string { sort.Strings(s); return strings.Join(s, ",") }
`
	if TextKey(a) != TextKey(b) {
		t.Error("import order split the vote")
	}
}

// The other half of the balance: normalisation must NOT approximate semantic
// equivalence. If it did, the text vote would start agreeing with candidates that
// merely behave alike, which is what execution measures — and the §3.5 comparison
// would be comparing behavioural clustering against itself.
func TestTextKeySeparatesRealDifferences(t *testing.T) {
	different := map[string]string{
		"different algorithm": `package solution

func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
`,
		"renamed local": `package solution

import "strings"

func Reverse(s string) string {
	var out strings.Builder
	for i := len(s) - 1; i >= 0; i-- {
		out.WriteByte(s[i])
	}
	return out.String()
}
`,
		"off by one": `package solution

import "strings"

func Reverse(s string) string {
	var b strings.Builder
	for i := len(s) - 2; i >= 0; i-- {
		b.WriteByte(s[i])
	}
	return b.String()
}
`,
	}
	want := TextKey(canonical)
	for name, src := range different {
		if got := TextKey(src); got == want {
			t.Errorf("%s: normalisation collapsed a real difference into the canonical key; "+
				"the text vote is approximating semantics and no longer contrasts with §3.5", name)
		}
	}
}

// An unparseable candidate must not pool with the other unparseable ones. Keying
// them all alike would manufacture a majority out of the failures — the mistake
// Behaviour avoids by keying refuted candidates on the refuting stage.
func TestUnparseableCandidatesDoNotPoolIntoAMajority(t *testing.T) {
	a := TextKey("this is not go at all")
	b := TextKey("neither is this, differently")
	if a == b {
		t.Fatal("two different unparseable candidates share a key; a run of garbage would " +
			"vote as a unanimous bloc")
	}
	// Byte-identical garbage should still agree, and whitespace should still fold.
	if TextKey("not go") != TextKey("not   go\n") {
		t.Error("whitespace split two identical unparseable candidates")
	}
}

func cand(i int, src string) Candidate { return Candidate{Index: i, Source: src} }

func TestTextVotePicksThePlurality(t *testing.T) {
	// 3 of the canonical answer (one reformatted, one with different comments), 2 of another.
	other := `package solution

func Reverse(s string) string { return s }
`
	cands := []Candidate{
		cand(0, canonical),
		cand(1, "package solution\nimport \"strings\"\nfunc Reverse(s string) string {\nvar b strings.Builder\nfor i := len(s) - 1; i >= 0; i-- {\nb.WriteByte(s[i])\n}\nreturn b.String()\n}\n"),
		cand(2, "// v3\n"+canonical),
		cand(3, other),
		cand(4, other),
	}
	winner, mass, ok := TextVote(cands)
	if !ok {
		t.Fatal("no vote")
	}
	if winner != 0 {
		t.Errorf("winner = %d, want 0 (the lowest index of the 3-member class)", winner)
	}
	if mass != 3.0/5.0 {
		t.Errorf("mass = %v, want 0.6", mass)
	}
}

// Raw mass, deliberately — see the TextVote doc. Invariant #9's Wilson bound
// applies to the ROUTING score, which crosses a calibrated threshold; arm (e)
// crosses no threshold, and bounding it below would handicap the foil.
func TestTextVoteReportsRawMassNotAWilsonBound(t *testing.T) {
	cands := []Candidate{cand(0, canonical), cand(1, canonical)}
	_, mass, ok := TextVote(cands)
	if !ok {
		t.Fatal("no vote")
	}
	if mass != 1.0 {
		t.Fatalf("unanimous 2-sample mass = %v, want exactly 1.0", mass)
	}
	if mass == UnanimousScore(2) {
		t.Error("TextVote returned the Wilson unanimous ceiling; it must report raw mass")
	}
}

// Determinism, and a tie-break independent of the order candidates happen to
// arrive in: arm (e) votes over batches drawn separately, so an order-sensitive
// tie-break would make the result depend on which batch finished first.
func TestTextVoteTieBreakIsOrderIndependent(t *testing.T) {
	other := "package solution\n\nfunc Reverse(s string) string { return s }\n"
	a := []Candidate{cand(0, canonical), cand(1, other)}
	b := []Candidate{cand(1, other), cand(0, canonical)}
	wa, _, _ := TextVote(a)
	wb, _, _ := TextVote(b)
	if wa != wb {
		t.Errorf("tie broken differently by input order: %d vs %d", wa, wb)
	}
	if wa != 0 {
		t.Errorf("tie winner = %d, want the lowest index 0", wa)
	}
}

func TestTextVoteEmpty(t *testing.T) {
	if _, _, ok := TextVote(nil); ok {
		t.Error("an empty candidate set reported a vote")
	}
}
