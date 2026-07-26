package main

import (
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
