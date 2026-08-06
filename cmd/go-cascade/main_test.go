package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cascade"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
	"github.com/scttfrdmn/go-cascade/internal/verify"
)

func bp(b bool) *bool { return &b }

// The concurrency benchmark exists for exactly one reason: to make the race rung
// fire. That rung is gated on a free AST predicate (verify.Static.UsesConcurrency),
// so a problem whose solutions carry no goroutine, channel, select or sync use
// SKIPS it — and a skip is scored OK, which is sound but measures nothing.
//
// This is not hypothetical. MultiPL-E Go has 0 concurrency problems, so the whole
// n=409 run (experiment 21) skipped the rung on all 488 records without a single
// test failing or a line of output saying so — the rung that caught the study's
// only confirmed judge over-acceptance never ran at the only large n. The failure
// mode is silent by construction, hence an explicit assertion.
//
// The reference is the right thing to check: it is the one program per problem
// known to be correct, so if IT does not trip the predicate, no candidate for that
// problem plausibly will either.
func TestConcurrencyBenchActuallyReachesTheRaceRung(t *testing.T) {
	const bench = "../../examples/bench/concurrency.jsonl"
	probs, err := readBench(bench)
	if err != nil {
		t.Fatalf("readBench(%s): %v", bench, err)
	}
	if len(probs) == 0 {
		t.Fatal("no problems: an empty concurrency set covers nothing")
	}

	refs, err := loadReferences("../../examples/bench", probs)
	if err != nil {
		t.Fatalf("loadReferences: %v", err)
	}
	// -refs is load-bearing here (invariant #4): without a reference the oracle
	// soundness check cannot run, and a paired run over these ids would be
	// calibrating against unvalidated generated tests.
	if len(refs) != len(probs) {
		t.Errorf("references %d/%d; every concurrency problem needs one so -refs can gate oracle soundness",
			len(refs), len(probs))
	}

	for _, p := range probs {
		src, ok := refs[p.ID]
		if !ok {
			t.Errorf("%s: no reference solution", p.ID)
			continue
		}
		st, err := verify.Analyse(src)
		if err != nil {
			t.Errorf("%s: reference does not parse: %v", p.ID, err)
			continue
		}
		if !st.UsesConcurrency {
			t.Errorf("%s: reference does not trip UsesConcurrency, so the race rung would be "+
				"SKIPPED (skips score OK — this problem contributes no race coverage)", p.ID)
		}
	}

	// The other direction, and the one that rots: concurrency.jsonl is a hand-curated
	// SUBSET of the hand-written benchmark, so adding a concurrency problem to
	// problems.jsonl / hard/ / scale/ without adding it here silently shrinks the
	// coverage set. Nothing else would notice — this run reports over its own n.
	have := make(map[string]bool, len(probs))
	for _, p := range probs {
		have[p.ID] = true
	}
	var missing []string
	err = filepath.WalkDir("../../examples/bench", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "solution.go" {
			return err
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		st, aerr := verify.Analyse(string(src))
		if aerr != nil {
			return nil // a non-parsing reference is a different bug, caught elsewhere
		}
		if id := filepath.Base(filepath.Dir(path)); st.UsesConcurrency && !have[id] {
			missing = append(missing, id)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk references: %v", err)
	}
	if len(missing) > 0 {
		t.Errorf("concurrency references absent from %s: %v — add them or the race-coverage "+
			"run silently under-reports its own scope", bench, missing)
	}
}

func TestCountJudgeErrors(t *testing.T) {
	// tier 0: judge PASS, truth wrong  -> false accept
	// tier 1: judge FAIL, truth correct -> false reject
	// tier 2: judge PASS, truth correct -> agree
	// tier 3: no truth recorded         -> skipped
	exec := calibrate.Record{Tiers: []calibrate.TierObs{
		{TrueCorrect: bp(false)},
		{TrueCorrect: bp(true)},
		{TrueCorrect: bp(true)},
		{TrueCorrect: nil},
	}}
	judge := calibrate.Record{Tiers: []calibrate.TierObs{
		{Correct: true},
		{Correct: false},
		{Correct: true},
		{Correct: false},
	}}
	fa, fr := countJudgeErrors(exec, judge)
	if fa != 1 {
		t.Errorf("false acceptances = %d, want 1", fa)
	}
	if fr != 1 {
		t.Errorf("false rejections = %d, want 1", fr)
	}
}

func TestArmRecordPath(t *testing.T) {
	cases := []struct {
		recOut, arm, want string
	}{
		{"records.json", "judge", "records.judge.json"},
		{"records.json", "execution", "records.execution.json"},
		{"out/pilot.json", "judge", "out/pilot.judge.json"},
		{"records", "judge", "records.judge.json"}, // no extension -> default .json
		{"a.b.json", "execution", "a.b.execution.json"},
	}
	for _, c := range cases {
		if got := armRecordPath(c.recOut, c.arm); got != c.want {
			t.Errorf("armRecordPath(%q, %q) = %q, want %q", c.recOut, c.arm, got, c.want)
		}
	}
}

// loadRecords is the read side of -resume: a missing file is not an error
// (nothing recorded yet), a written file round-trips, and a malformed one errors
// rather than silently discarding a partial run's expensive records. See #21.
func TestLoadRecordsRoundTripAndMissing(t *testing.T) {
	dir := t.TempDir()

	// Missing file: no error, no records (the fresh-start case).
	if recs, err := loadRecords(filepath.Join(dir, "absent.json")); err != nil || recs != nil {
		t.Errorf("missing file: got (%v, %v), want (nil, nil)", recs, err)
	}

	// Round-trip: what a checkpoint wrote is what resume reads back, so the
	// done-set is reconstructed correctly.
	path := filepath.Join(dir, "recs.json")
	want := []calibrate.Record{{ID: "a"}, {ID: "b", OracleUnsound: true}}
	if err := writeJSONFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" || !got[1].OracleUnsound {
		t.Errorf("round-trip mismatch: got %+v", got)
	}

	// Malformed file: an error, so a resume never silently starts from scratch and
	// overwrites a partial run.
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRecords(bad); err == nil {
		t.Error("malformed records file must error, not silently return empty")
	}
}

// writeJSONFile must be atomic: overwriting an existing checkpoint leaves a
// complete, parseable file and no stray temp files. A non-atomic writer
// truncates in place, so an external SIGTERM mid-write can leave a 0-byte file
// and lose an entire long run's progress — the failure that defeated -resume on
// the estimator run. This asserts the post-write invariants the fix guarantees.
func TestWriteJSONFileAtomicOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recs.json")

	// A first checkpoint, then a second (larger) one over it — the overwrite path.
	if err := writeJSONFile(path, []calibrate.Record{{ID: "old"}}); err != nil {
		t.Fatal(err)
	}
	want := []calibrate.Record{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if err := writeJSONFile(path, want); err != nil {
		t.Fatal(err)
	}

	// The target parses and holds the new content, not a truncation of the old.
	got, err := loadRecords(path)
	if err != nil {
		t.Fatalf("checkpoint did not parse after overwrite: %v", err)
	}
	if len(got) != 3 || got[0].ID != "a" || got[2].ID != "c" {
		t.Errorf("overwrite left wrong content: got %+v", got)
	}

	// No temp file is left behind: os.Rename consumed it. A lingering ".tmp-*"
	// would mean the rename never happened and the write was not atomic.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "recs.json" {
			t.Errorf("stray file left in checkpoint dir: %q (temp not renamed away)", e.Name())
		}
	}

	// The mode matches the old os.WriteFile path. os.CreateTemp defaults to 0600,
	// so without an explicit chmod the atomic writer would silently tighten the
	// permissions of every records and certificate file the tool emits.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o644 {
		t.Errorf("checkpoint mode = %#o, want 0644 (must match the pre-atomic writer)", got)
	}
}

// The -seed-kind flag names three distinct defect classes and an unrecognised
// value must be rejected rather than silently defaulting to logic: a run that
// reported logic-mutant η_fa under a "race" heading would be a wrong published
// number, not a usability annoyance.
func TestParseSeedKind(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want cascade.SeedKind
		ok   bool
	}{
		{"logic", cascade.SeedLogic, true},
		{"race", cascade.SeedRace, true},
		{"scar-free-race", cascade.SeedScarFreeRace, true},
		{"", cascade.SeedLogic, false},
		{"scarfree", cascade.SeedLogic, false},
		{"Race", cascade.SeedLogic, false},
	} {
		got, ok := parseSeedKind(tc.in)
		if ok != tc.ok {
			t.Errorf("parseSeedKind(%q) ok = %v, want %v", tc.in, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Errorf("parseSeedKind(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	// The three kinds must stay distinct — aliasing two of them would pool the
	// scar-bearing and scar-free rates, which is the one comparison this exists for.
	seen := map[cascade.SeedKind]string{}
	for _, s := range []string{"logic", "race", "scar-free-race"} {
		k, _ := parseSeedKind(s)
		if prev, dup := seen[k]; dup {
			t.Errorf("%q and %q map to the same SeedKind", prev, s)
		}
		seen[k] = s
	}
}

// The record's Kind uses the FLAG spelling and the report uses prose. Both are
// wanted — a record has to be matchable back to the command that produced it, and
// a table has to say "sync-deletion race" rather than "race" — but they must stay
// distinguishable, because the resume guard compares a file's Kind against
// cascade.SeedKindName. If prose ever leaked into a record, resuming a real run
// would abort claiming the file held a different defect class.
func TestSeedKindNamesAreFlagSpellingsAndDistinctFromProse(t *testing.T) {
	for _, s := range []string{"logic", "race", "scar-free-race"} {
		k, ok := parseSeedKind(s)
		if !ok {
			t.Fatalf("parseSeedKind(%q) rejected its own spelling", s)
		}
		if got := cascade.SeedKindName(k); got != s {
			t.Errorf("SeedKindName(%v) = %q, want the flag spelling %q — a record's kind must round-trip through -seed-kind", k, got, s)
		}
	}
	// Prose labels must be distinct from each other too: pooling a scar-free rate
	// with a sync-deletion one under a shared heading is the failure both names guard.
	seen := map[string]bool{}
	for _, k := range []cascade.SeedKind{cascade.SeedLogic, cascade.SeedRace, cascade.SeedScarFreeRace} {
		p := seedKindName(k)
		if seen[p] {
			t.Errorf("two seed kinds share the prose label %q", p)
		}
		seen[p] = true
	}
}

// tallySeeded must derive the table from the records, so the printed rate and the
// persisted one cannot drift. Experiment 30 accumulated tallies alongside the run
// and persisted nothing, which is how its per-problem seed counts ended up
// reconstructed from a stderr log.
func TestTallySeededDerivesFromRecords(t *testing.T) {
	levels := []prompt.JudgeStrictness{prompt.JudgeStrict, prompt.JudgePermissive}
	recs := []cascade.SeededRecord{
		{ID: "a", Kind: "scar-free-race", Seeds: 2, Levels: []cascade.SeededVerdict{
			{Level: prompt.JudgeStrict, Judged: 2, FalseAccept: 0},
			{Level: prompt.JudgePermissive, Judged: 2, FalseAccept: 1},
		}},
		// A harvest that yielded nothing must not inflate the problem denominator: a
		// zero over more problems reads as stronger evidence than it is.
		{ID: "b", Kind: "scar-free-race", Seeds: 0},
		{ID: "c", Kind: "scar-free-race", Seeds: 1, Levels: []cascade.SeededVerdict{
			{Level: prompt.JudgeStrict, Judged: 1, FalseAccept: 0},
			{Level: prompt.JudgePermissive, Judged: 1, FalseAccept: 1},
		}},
	}
	recs[1].NoSeed = "no mutant compiled and was refuted"
	totals, totalWrong, used := tallySeeded(recs, levels)
	if totalWrong != 3 {
		t.Errorf("harvested seeds = %d, want 3", totalWrong)
	}
	if used != 2 {
		t.Errorf("problems used = %d, want 2 (the empty harvest must not count)", used)
	}
	if got := totals[prompt.JudgeStrict]; got.Judged != 3 || got.FalseAccept != 0 {
		t.Errorf("strict = %+v, want judged 3 / false-accept 0", got)
	}
	if got := totals[prompt.JudgePermissive]; got.Judged != 3 || got.FalseAccept != 2 {
		t.Errorf("permissive = %+v, want judged 3 / false-accept 2", got)
	}
}

// A row whose judging stopped part-way is kept on disk — its verdicts were paid
// for and are real — but must NOT be skipped on resume. Skipping it would leave the
// file permanently short of the seeds it says it harvested, and the shortfall is
// invisible in an aggregate table: the denominator just reads smaller.
func TestSeededCompleteRejectsPartialRows(t *testing.T) {
	full := cascade.SeededRecord{
		Seeds:   2,
		Mutants: []cascade.SeededMutant{{Desc: "x"}, {Desc: "y"}},
		Levels: []cascade.SeededVerdict{
			{Level: prompt.JudgeStrict, Judged: 2},
			{Level: prompt.JudgePermissive, Judged: 2},
		},
	}
	if !seededComplete(full) {
		t.Error("a fully-judged row was treated as partial; resume would redo paid work")
	}
	// Judging died after the first mutant.
	short := full
	short.Mutants = full.Mutants[:1]
	short.Levels = []cascade.SeededVerdict{{Level: prompt.JudgeStrict, Judged: 1}}
	if seededComplete(short) {
		t.Error("a partially-judged row was treated as complete; resume would skip it forever")
	}
	// Every mutant harvested, but one level never ran.
	lopsided := full
	lopsided.Levels = []cascade.SeededVerdict{
		{Level: prompt.JudgeStrict, Judged: 2},
		{Level: prompt.JudgePermissive, Judged: 1},
	}
	if seededComplete(lopsided) {
		t.Error("a row missing one level's verdicts was treated as complete")
	}
	// No levels at all: the harvest happened, nothing was judged.
	if seededComplete(cascade.SeededRecord{Seeds: 1, Mutants: []cascade.SeededMutant{{}}}) {
		t.Error("a row with no verdicts at all was treated as complete")
	}
	// A zero-seed row IS finished — the harvest ran and found nothing. Re-running it
	// on resume would redraw a tier for a problem already known to yield no seed,
	// which on a paid arm is spend for a result already on disk.
	if !seededComplete(cascade.SeededRecord{ID: "x", NoSeed: "no mutant compiled and was refuted"}) {
		t.Error("a zero-seed row was treated as partial; resume would pay to redraw it")
	}
}

// The seeded record must carry the fields whose absence made experiment 30's
// defect classes unrecoverable, and it must survive the round-trip through the
// checkpoint writer — a resumable run reads back what it wrote.
func TestSeededRecordRoundTripsWithSourcesAndOperators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seeded.json")
	want := []cascade.SeededRecord{{
		ID: "conc_safe_counter", Kind: "scar-free-race", Seeds: 1,
		Mutants: []cascade.SeededMutant{{
			Desc: "escape-defer: move shared write outside the deferred-unlock region",
			// The program itself. This is the field the write-up needed and did not have.
			Source:       "package main\n\nfunc main() {}\n",
			DataRace:     true,
			PlainRefuted: false,
			Verdicts:     map[prompt.JudgeStrictness]bool{prompt.JudgeStrict: false},
		}},
		Levels: []cascade.SeededVerdict{{Level: prompt.JudgeStrict, Judged: 1, FalseAccept: 0}},
	}}
	if err := writeJSONFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadSeededRecords(path)
	if err != nil {
		t.Fatalf("loadSeededRecords: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	m := got[0].Mutants[0]
	if m.Source != want[0].Mutants[0].Source {
		t.Error("mutant source did not survive the round-trip; the forensic follow-up is lost")
	}
	if m.Desc == "" {
		t.Error("operator description did not survive; per-operator seed counts are unrecoverable")
	}
	// DataRace is a hard filter on the scar-free arm and forensic on the deletion
	// arm, so an arm-comparability claim rests on it being persisted, not inferred.
	if !m.DataRace {
		t.Error("DataRace did not survive the round-trip")
	}
	if pass, ok := m.Verdicts[prompt.JudgeStrict]; !ok || pass {
		t.Errorf("per-mutant verdict did not survive: %+v", m.Verdicts)
	}
	if got[0].Kind != "scar-free-race" {
		t.Errorf("kind = %q; the defect class must travel with the numbers", got[0].Kind)
	}
	// A missing file is a cold start, not an error: -resume on the first run must work.
	empty, err := loadSeededRecords(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || empty != nil {
		t.Errorf("absent records file: got (%v, %v), want (nil, nil)", empty, err)
	}
}

// The paired comparison must be printed on ONE denominator. The live n=409 pass
// printed the text arm over all voted rows (296/409 = 0.724) against the cluster
// over non-abstained rows only (335/366 = 0.915), which reads as a 0.19 gap where
// the paired figure is 0.11 (295/366 vs 335/366). Nothing failed — both numbers
// were individually correct — so this asserts the shape of the report, not a value.
func TestSelfConsistencySummaryPairsOnOneDenominator(t *testing.T) {
	// 4 rows: two where both answer, two where the cluster abstains. The text arm is
	// right on one abstention row, so an all-rows text rate (3/4) differs from the
	// paired one (2/2) — if the report pooled them the assertions below would catch it.
	obs := []cascade.SelfConsistencyObs{
		{Fanout: 5, TextCorrect: true, ClusterCorrect: true, Agreed: true},
		{Fanout: 5, TextCorrect: true, ClusterCorrect: true, Agreed: true},
		{Fanout: 5, TextCorrect: true, ClusterAbstained: true},
		{Fanout: 5, TextCorrect: false, ClusterAbstained: true},
	}
	out := captureOutput(t, func() { printSelfConsistencySummary(obs, 1, 1, 1) })

	// The paired line covers the 2 rows both selectors answered, both arms over 2.
	if !strings.Contains(out, "both selectors answered") {
		t.Error("no paired-denominator line; the two arms may be printed over different n")
	}
	for _, want := range []string{
		"arm (e), text vote:        2/2",
		"§3.5, behavioural cluster: 2/2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing paired count %q\nout:\n%s", want, out)
		}
	}
	// The all-rows text rate may still be shown, but only labelled as incomparable.
	if strings.Contains(out, "3/4") && !strings.Contains(out, "NOT") {
		t.Error("all-rows text rate printed without the incomparability caveat")
	}
	// The abstention rows get their own denominator, never folded into either rate.
	if !strings.Contains(out, "abstained on, the text vote was correct 1/2") {
		t.Errorf("abstention-row text rate missing or pooled\nout:\n%s", out)
	}
}

// A run where the cluster abstains on everything has no paired comparison. Printing
// a text-only rate there would read as a selector result with no baseline.
func TestSelfConsistencySummaryRefusesAnEmptyPairing(t *testing.T) {
	obs := []cascade.SelfConsistencyObs{
		{Fanout: 5, TextCorrect: true, ClusterAbstained: true},
		{Fanout: 5, TextCorrect: false, ClusterAbstained: true},
	}
	out := captureOutput(t, func() { printSelfConsistencySummary(obs, 1, 1, 1) })
	if !strings.Contains(out, "no paired comparison") {
		t.Errorf("all-abstain run did not say the pairing is empty\nout:\n%s", out)
	}
	if strings.Contains(out, "behavioural cluster:") {
		t.Error("printed a cluster rate with zero non-abstained rows")
	}
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	os.Stderr = old
	return <-done
}

// A live run's timeout tally must print even at zero, and printCert's must not.
// The asymmetry is load-bearing rather than cosmetic: TierObs.TimedOut is
// omitempty, so in a records file replayed from disk "0 timeouts" and "collected
// before timeouts were recorded" are the same bytes — printing "0" there would
// assert something the file cannot support. A live run just produced the records,
// so its zero is a measurement (issue #63).
func TestTimeoutTallyPrintsZeroOnlyWhenZeroIsAMeasurement(t *testing.T) {
	clean := []calibrate.Record{
		{ID: "a", Tiers: []calibrate.TierObs{{Tier: "cheap"}}},
		{ID: "b", Tiers: []calibrate.TierObs{{Tier: "cheap"}}},
	}
	out := captureStderr(t, func() { reportTimeouts("timeouts", clean) })
	if !strings.Contains(out, "0/2") {
		t.Errorf("a live run must report its zero tally; got %q", out)
	}
	if strings.Contains(out, "WARNING") {
		t.Errorf("a clean run warned about timeouts: %q", out)
	}

	slow := append([]calibrate.Record{}, clean...)
	slow = append(slow, calibrate.Record{ID: "c",
		Tiers: []calibrate.TierObs{{Tier: "cheap", TimedOut: true}}})
	out = captureStderr(t, func() { reportTimeouts("timeouts", slow) })
	if !strings.Contains(out, "WARNING") || !strings.Contains(out, "1/3") {
		t.Errorf("a run with a timeout must warn and give the count; got %q", out)
	}
	// The warning must say the refutation stands, or a reader will "clean" the
	// records — which selects the sample on an outcome (invariants #4, #8).
	if !strings.Contains(out, "KEPT") {
		t.Errorf("the warning does not say the timed-out records are kept: %q", out)
	}

	// printCert: silent at zero, explicit above it.
	cert := &calibrate.Certificate{Valid: true, Alpha: 0.1, Delta: 0.1, N: 3}
	out = captureOutput(t, func() { printCert("/tmp/x.json", cert) })
	if strings.Contains(out, "timeout") {
		t.Errorf("printCert mentioned timeouts at NTimedOut=0, where 0 may mean 'never recorded': %q", out)
	}
	cert.NTimedOut = 1
	out = captureOutput(t, func() { printCert("/tmp/x.json", cert) })
	if !strings.Contains(out, "timeouts") || !strings.Contains(out, "1 of 3") {
		t.Errorf("printCert hid a nonzero timeout count: %q", out)
	}
}
