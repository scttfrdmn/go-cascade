package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cascade"
)

func bp(b bool) *bool { return &b }

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
