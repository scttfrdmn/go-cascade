package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
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
}
