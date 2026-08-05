package verify

import (
	"bytes"
	"go/printer"
	"strings"
	"testing"
)

// TestScarFreeMutantsLeaveNoFormattingScar is the layout half of "scar-free."
//
// The operators are designed so nothing is missing from the page — every lock is
// paired, every Wait still runs. But go/printer lays a block out from its
// statements' RECORDED SOURCE LINES, not from their slice order, so any operator
// that reorders statements makes those lines run backwards and the printer emits
// a blank line at exactly the mutation site. A stray blank line where the edit
// happened is a scar, and gofmt does not remove it: gofmt preserves author blank
// lines between statements rather than normalising them away.
//
// This was measured, not hypothesised. Before clearPositions, the plain escape
// operator gained one blank line on hard_conc_once_init and the deferred-escape
// operator gained two on hard_conc_rate_limiter — 3 of 17 sites carrying a
// positional tell, all of them from the reordering operators (the downgrade and
// defer-wait operators edit in place and never did).
//
// The assertion is a blank-line COUNT rather than a diff because the count is
// what a reader's eye catches and because it is robust to the intended edit: no
// scar-free operator is supposed to add or remove a blank line anywhere. Note
// the deliberate asymmetry with the seed tests — this one is AST-and-printer
// only, no toolchain, so it runs in the short suite and guards every site rather
// than only the sites that go on to become seeds.
func TestScarFreeMutantsLeaveNoFormattingScar(t *testing.T) {
	blanks := func(s string) int {
		n := 0
		for _, l := range strings.Split(s, "\n") {
			if strings.TrimSpace(l) == "" {
				n++
			}
		}
		return n
	}

	checked := 0
	for _, id := range concurrencyRefIDs {
		src := loadConcurrencyRef(t, id)
		sites, fset, f, err := collectScarFreeRaceSites(src)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		// Compare against the printer's own round-trip of the unmutated file, not
		// against the source: printer.Fprint reformats struct-field alignment with
		// tabs, so the source is not its own fixed point and every mutant would
		// look different for a reason that has nothing to do with the mutation.
		var base bytes.Buffer
		if err := printer.Fprint(&base, fset, f); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		want := blanks(base.String())

		for _, m := range sites {
			m.apply()
			var buf bytes.Buffer
			perr := printer.Fprint(&buf, fset, f)
			m.revert()
			if perr != nil {
				t.Errorf("%s: printing mutant %q: %v", id, m.desc, perr)
				continue
			}
			checked++
			if got := blanks(buf.String()); got != want {
				t.Errorf("%s: mutant %q changed the blank-line count %d -> %d.\n"+
					"A reordering operator must clear the moved statements' positions "+
					"(see clearPositions) or go/printer pads at the mutation site, which "+
					"is a positional scar in an arm whose premise is that there is none.",
					id, m.desc, want, got)
			}
		}

		// Reverting must restore the file exactly, or a later site in the same file
		// is printed against a corrupted AST — the mutants would differ from what
		// their descriptions claim and the harvest would be measuring the wrong
		// programs. clearPositions returns an undo closure for this reason.
		var after bytes.Buffer
		if err := printer.Fprint(&after, fset, f); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if after.String() != base.String() {
			t.Errorf("%s: reverting all sites did not restore the AST", id)
		}
	}
	if checked == 0 {
		t.Fatal("no scar-free sites were checked — the operators reach nothing")
	}
}

// TestDeferredEscapeVetoesControlFlowExits pins the veto that keeps the
// deferred-escape operator from manufacturing DEADLOCKS instead of races.
//
// Undeferring an Unlock only unlocks on the fall-through path. If the region the
// defer covered contains a `return`, that path leaks the lock and the mutant
// hangs — a defect class this arm must not contain, because a deadlock is refuted
// by a timeout (whose cause can be external to the candidate) and because a Lock
// with a `return` before its Unlock is a *visible* imbalance, which is what the
// deletion operator already tests.
//
// The two cases are the same function shape differing only in the return, so a
// veto that fired for any other reason would fail the second case too. The
// FuncLit case is here because vetoing on a literal's return would be
// over-strict: it exits the literal, not the enclosing critical section, and
// these operators have little enough reach already.
func TestDeferredEscapeVetoesControlFlowExits(t *testing.T) {
	const preamble = `package solution

import "sync"

type S struct {
	mu sync.Mutex
	a  int
	b  int
}

`
	cases := []struct {
		name string
		body string
		want int // deferred-escape sites expected
	}{{
		name: "no exit: site is offered",
		body: `func (s *S) F(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.a += n
	s.b += n
}
`,
		want: 1,
	}, {
		name: "return inside the covered region: vetoed",
		body: `func (s *S) F(n int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.a > n {
		return true
	}
	s.b += n
	return false
}
`,
		want: 0,
	}, {
		name: "return inside a func literal: not an exit, site is offered",
		body: `func (s *S) F(n int, once *sync.Once) {
	s.mu.Lock()
	defer s.mu.Unlock()
	once.Do(func() {
		if n < 0 {
			return
		}
		s.a = n
	})
	s.b += n
}
`,
		want: 1,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sites, _, _, err := collectScarFreeRaceSites(preamble + tc.body)
			if err != nil {
				t.Fatal(err)
			}
			got := 0
			for _, s := range sites {
				if strings.Contains(s.desc, "undeferring the Unlock") {
					got++
				}
			}
			if got != tc.want {
				t.Errorf("deferred-escape sites = %d, want %d", got, tc.want)
				for _, s := range sites {
					t.Logf("  site: %s", s.desc)
				}
			}
		})
	}
}
