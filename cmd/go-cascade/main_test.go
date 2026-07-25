package main

import "testing"

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
