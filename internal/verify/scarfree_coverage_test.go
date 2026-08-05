package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// concurrencyRefIDs mirrors examples/bench/concurrency.jsonl. Duplicated rather
// than parsed because this package must not import the CLI's bench reader, and
// because a drifting list is caught below: every id must resolve to a reference.
var concurrencyRefIDs = []string{
	"conc_parallel_map", "conc_parallel_sum", "conc_safe_counter", "conc_parallel_filter",
	"conc_fan_in_merge", "conc_first_success", "conc_parallel_histogram", "conc_bounded_pipeline",
	"hard_conc_rate_limiter", "hard_conc_once_init", "hard_conc_ordered_fanout",
}

func loadConcurrencyRef(t *testing.T, id string) string {
	t.Helper()
	for _, base := range []string{"../../examples/bench/refs", "../../examples/bench/hard/refs"} {
		b, err := os.ReadFile(filepath.Join(base, id, "solution.go"))
		if err == nil {
			return string(b)
		}
	}
	t.Fatalf("%s: no reference solution under examples/bench", id)
	return ""
}

// The scar-free operators are only as good as the constructs they can find, and
// on THIS benchmark two of the three barely fire. Measured over the 11
// concurrency references (experiment 28): 15 AST sites, but only 6 survive the
// compile + `-race` filter to become usable seeds, and 5 of those 6 come from a
// single operator.
//
// That is a fact about the corpus, not about the judge, and it is exactly the
// shape of failure that reads as a scientific null if nobody checks: a seeded
// dangerous-mode sweep over these problems would report an η_fa over a
// denominator of 6, where a 0/6 result has a 95% upper bound of 0.393 — i.e. it
// could not distinguish "the judge catches scar-free races" from "six seeds
// cannot tell." The sweep was declined on this basis rather than run.
//
// This test pins the site counts so the situation cannot silently change in
// either direction. Adding RWMutex or multi-statement-critical-section problems
// to the benchmark SHOULD break it — that is the signal the sweep is worth
// re-pricing, and the failure message says so.
func TestScarFreeOperatorCoverageOnTheConcurrencyBenchmark(t *testing.T) {
	// AST-only: no compilation, no -race, so this runs in the short suite. The
	// seed counts (which need both) are recorded in results/, not asserted here.
	perOperator := map[string]int{}
	withSites := 0
	total := 0
	for _, id := range concurrencyRefIDs {
		src := loadConcurrencyRef(t, id)
		sites, _, _, err := collectScarFreeRaceSites(src)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if len(sites) > 0 {
			withSites++
		}
		total += len(sites)
		for _, s := range sites {
			switch {
			case strings.Contains(s.desc, "RLock/RUnlock"):
				perOperator["downgrade"]++
			case strings.Contains(s.desc, "out of the"):
				perOperator["escape"]++
			case strings.Contains(s.desc, "defer"):
				perOperator["defer-wait"]++
			default:
				t.Errorf("%s: unclassified site %q — a new operator needs a case here", id, s.desc)
			}
		}
	}

	const (
		wantTotal     = 15
		wantWithSites = 10
	)
	if total != wantTotal || withSites != wantWithSites {
		t.Errorf("scar-free coverage moved: %d sites over %d/%d problems, recorded %d over %d.\n"+
			"If the benchmark GAINED concurrency problems this is expected — re-measure the "+
			"seed count (see results/scarfree-coverage-n11.md) and re-price the seeded sweep, "+
			"because the decision not to run it rests on a denominator of 6.",
			total, withSites, len(concurrencyRefIDs), wantTotal, wantWithSites)
	}

	// The lopsidedness is the finding, so it is asserted per operator: a pooled
	// total would hide that one operator carries the set.
	for op, want := range map[string]int{"defer-wait": 8, "downgrade": 6, "escape": 1} {
		if perOperator[op] != want {
			t.Errorf("operator %s: %d sites, recorded %d", op, perOperator[op], want)
		}
	}

	// The downgrade operator needs an RWMutex to compile at all. There is not one
	// anywhere in the benchmark, so all 6 of its sites are discarded at build —
	// they cost a compile each and yield nothing. Pinned separately from the count
	// because the count alone does not say the sites are DEAD.
	for _, id := range concurrencyRefIDs {
		if strings.Contains(loadConcurrencyRef(t, id), "sync.RWMutex") {
			t.Errorf("%s now uses sync.RWMutex: the downgrade operator can finally produce a "+
				"compiling mutant, so the scar-free seed count should be re-measured", id)
		}
	}
}
