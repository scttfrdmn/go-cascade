package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// concurrencyRefIDs mirrors examples/bench/concurrency.jsonl. Duplicated rather
// than parsed because this package must not import the CLI's bench reader, and
// because a drifting list is caught below: every id must resolve to a reference,
// and TestConcurrencyRefIDsMatchTheBenchFile checks the count against the file.
var concurrencyRefIDs = []string{
	"conc_parallel_map", "conc_parallel_sum", "conc_safe_counter", "conc_parallel_filter",
	"conc_fan_in_merge", "conc_first_success", "conc_parallel_histogram", "conc_bounded_pipeline",
	"hard_conc_rate_limiter", "hard_conc_once_init", "hard_conc_ordered_fanout",
}

// TestConcurrencyRefIDsMatchTheBenchFile closes the hole that made every other
// guard in this file blind to the extension the write-up recommends. The id list
// above is hardcoded, so adding a 12th concurrency problem to the benchmark left
// all the site/seed/RWMutex assertions passing on the stale 11 — the exact signal
// ("this extension breaks the test on purpose, go re-price the sweep") could not
// fire on the one path anyone would take to trigger it.
//
// Compares only the count, and against a line count rather than a parse, because
// this package must not import the CLI's bench reader. That is enough: any added or
// removed problem moves it.
func TestConcurrencyRefIDsMatchTheBenchFile(t *testing.T) {
	b, err := os.ReadFile("../../examples/bench/concurrency.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	if n != len(concurrencyRefIDs) {
		t.Errorf("concurrency.jsonl has %d problems, concurrencyRefIDs has %d.\n"+
			"Add the new id(s) here, then re-measure the scar-free seed count and "+
			"re-price the declined sweep (results/scarfree-coverage-n11.md): the "+
			"decline rests on a denominator of 9 against a registered bar of 10.",
			n, len(concurrencyRefIDs))
	}
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

func concurrencyRefDir(t *testing.T, id string) string {
	t.Helper()
	for _, base := range []string{"../../examples/bench/refs", "../../examples/bench/hard/refs"} {
		d := filepath.Join(base, id)
		if _, err := os.Stat(filepath.Join(d, "solution.go")); err == nil {
			return d
		}
	}
	t.Fatalf("%s: no reference directory under examples/bench", id)
	return ""
}

// The scar-free operators are only as good as the constructs they can find, so
// their reach on this benchmark is a measured quantity that decides whether a
// paid sweep is worth funding. Measured over the 11 concurrency references
// (experiment 28): 15 AST sites yielding 9 seeds that compile AND draw an actual
// ThreadSanitizer report.
//
// This test pins the AST site counts. It is AST-only — no compilation, no -race —
// so it runs in the short suite; the seed counts are pinned separately by
// TestScarFreeSeedCountOnTheConcurrencyBenchmark, which needs the toolchain.
//
// Both numbers are pinned because experiment 28's first pass got the seed count
// wrong while the site count was right: the RWMutex downgrade operator rewrote its
// call sites and left the declaration a sync.Mutex, so all 6 of its sites compiled
// away to nothing and the shortfall was published as a property of the corpus
// ("no sync.RWMutex in examples/bench") rather than of the operator. Sites alone
// cannot catch that class of bug — every site was found, and none survived.
func TestScarFreeOperatorCoverageOnTheConcurrencyBenchmark(t *testing.T) {
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
			perOperator[classifyScarFreeSite(t, id, s.desc)]++
		}
	}

	const (
		wantTotal     = 16
		wantWithSites = 10
	)
	if total != wantTotal || withSites != wantWithSites {
		t.Errorf("scar-free coverage moved: %d sites over %d/%d problems, recorded %d over %d.\n"+
			"If the benchmark GAINED concurrency problems this is expected — re-measure the "+
			"seed count (see results/scarfree-coverage-n11.md) and re-price the seeded sweep, "+
			"because the decision not to run it rests on a denominator of 9.",
			total, withSites, len(concurrencyRefIDs), wantTotal, wantWithSites)
	}

	for op, want := range map[string]int{
		"defer-wait": 8, "downgrade": 6, "escape": 1, "escape-defer": 1,
	} {
		if perOperator[op] != want {
			t.Errorf("operator %s: %d sites, recorded %d", op, perOperator[op], want)
		}
	}
}

// classifyScarFreeSite bins a site description by operator, failing on anything it
// cannot place. The order matters and is the reverse of the obvious one: the
// downgrade check runs first because its description also contains "declaration",
// and the escape check before the defer one because a future deferred-form escape
// operator would mention both. A new operator must add a case here rather than
// landing in an existing bin — an unclassified site is an error, not a default,
// because a silently mis-binned site would corrupt the per-operator tallies that
// are the whole point of asserting them separately.
func classifyScarFreeSite(t *testing.T, id, desc string) string {
	t.Helper()
	switch {
	case strings.Contains(desc, "RLock/RUnlock"):
		return "downgrade"
	// The two escape forms are binned SEPARATELY. They differ in what they have
	// to edit — the deferred form must convert `defer mu.Unlock()` to a plain
	// call, the other only reorders — so pooling them would hide one form dying
	// while the other carried the count, which is exactly the failure mode that
	// let a broken downgrade operator be reported as a fact about the corpus.
	// This check precedes the plain-escape one because its description contains
	// both substrings.
	case strings.Contains(desc, "undeferring the Unlock"):
		return "escape-defer"
	case strings.Contains(desc, "out of the"):
		return "escape"
	case strings.Contains(desc, "defer"):
		return "defer-wait"
	}
	t.Errorf("%s: unclassified site %q — a new operator needs a case here", id, desc)
	return "?"
}

// TestScarFreeSeedCountOnTheConcurrencyBenchmark pins the number the funding
// decision actually rests on. Sites are cheap to count and decide nothing; a
// *seed* is a mutant that compiles and draws a real ThreadSanitizer report, and
// only seeds can be judged.
//
// This exists because the repo has been burned by exactly the inverse — asserting
// a gate's existence rather than that it fires (the -race rung was absent from all
// 488 records of experiment 21 and nothing failed). A site count of 15 was true
// throughout the period when the operator set produced 6 seeds instead of 9.
//
// It shells out to the toolchain (~20 s), so it is skipped in the short suite.
func TestScarFreeSeedCountOnTheConcurrencyBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("harvests mutants: compiles and runs -race per site")
	}
	r, err := NewRunner("", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Recorded per problem so a change points at the problem that moved, and so
	// the total cannot drift while offsetting errors cancel.
	want := map[string]int{
		"conc_parallel_map": 0, "conc_parallel_sum": 1, "conc_safe_counter": 0,
		"conc_parallel_filter": 1, "conc_fan_in_merge": 2, "conc_first_success": 1,
		"conc_parallel_histogram": 1, "conc_bounded_pipeline": 0, "hard_conc_rate_limiter": 2,
		"hard_conc_once_init": 2, "hard_conc_ordered_fanout": 0,
	}
	wantByOperator := map[string]int{
		"defer-wait": 4, "downgrade": 4, "escape": 1, "escape-defer": 1,
	}

	got := map[string]int{}
	gotByOperator := map[string]int{}
	raceOnly := map[string]int{} // seeds NOT refuted by a plain (no -race) run
	total := 0
	for _, id := range concurrencyRefIDs {
		dir := concurrencyRefDir(t, id)
		src := loadConcurrencyRef(t, id)
		var tests strings.Builder
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if !strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			tests.Write(b)
		}
		// Budget above the site count so nothing is sampled away: this is a census.
		seeds, err := ScarFreeRaceKilledMutants(context.Background(), r, src,
			tests.String(), "", 20, 3, 60*time.Second)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		got[id] = len(seeds)
		total += len(seeds)
		for _, m := range seeds {
			op := classifyScarFreeSite(t, id, m.Desc)
			gotByOperator[op]++
			if !m.PlainRefuted {
				raceOnly[op]++
			}
			// Every seed must carry a ThreadSanitizer report: this arm's filter is
			// DataRace, not merely "failed under -race". A mutant that returns the
			// wrong answer deterministically is a different defect class wearing
			// this operator's clothes, and experiment 28's first pass counted one
			// (a deferred wg.Wait() on conc_safe_counter, no race at all).
			if !m.DataRace {
				t.Errorf("%s: seed %q has no DATA RACE — the scar-free filter leaked",
					id, m.Desc)
			}
		}
	}

	// 10, which is exactly the bar experiment 28 registered before its harvest —
	// the deferred-escape operator supplied the tenth. The guard below is what
	// made that happen without anyone re-reading the write-up: it was pinned at 9
	// and FAILED when the operator landed, which is the whole design (see the
	// doc comment).
	const wantTotal = 10
	if total != wantTotal {
		t.Errorf("scar-free SEED count moved: %d, recorded %d.\n"+
			"This is the number the seeded sweep's power rests on (results/"+
			"scarfree-coverage-n11.md and results/deferred-escape-n11.md): the "+
			"registered bar was >=10 seeds and the corrected harvest returns 10. "+
			"If this DROPPED below 10 the sweep no longer clears its bar; if it "+
			"rose, re-run the power arithmetic before quoting any rate.",
			total, wantTotal)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: %d seeds, recorded %d", id, got[id], w)
		}
	}
	for op, w := range wantByOperator {
		if gotByOperator[op] != w {
			t.Errorf("operator %s: %d seeds, recorded %d", op, gotByOperator[op], w)
		}
	}

	// Experiment 28's Result 3 was that all 9 seeds were ALSO refuted without
	// -race, so the rung was not load-bearing for any of them — a seeded sweep
	// could only ever have measured whether a reader notices a defect the plain
	// test run already catches. The deferred-escape seed is the first that is
	// not: undeferring the Unlock widens the window enough that the ordinary run
	// passes and only ThreadSanitizer objects.
	//
	// Pinned per operator rather than as a total because that is the claim: the
	// escape-defer operator is the one that produces -race-only seeds, and a
	// total of 1 could be satisfied by any operator drifting into the bucket.
	// PlainRefuted is recorded and never filtered (it must not be — filtering one
	// arm breaks comparability with the scar-bearing arm), so this is the only
	// place the asymmetry is asserted.
	wantRaceOnly := map[string]int{
		"defer-wait": 0, "downgrade": 0, "escape": 0, "escape-defer": 1,
	}
	for op, w := range wantRaceOnly {
		if raceOnly[op] != w {
			t.Errorf("operator %s: %d seeds refuted ONLY under -race, recorded %d.\n"+
				"If this fell to 0 for escape-defer the sweep is back to measuring "+
				"reader-visibility on defects a plain run already catches; if it rose "+
				"elsewhere, say so in the write-up — it strengthens the arm.",
				op, raceOnly[op], w)
		}
	}
}
