package prompt

import "testing"

func TestParseSpec(t *testing.T) {
	reply := "Here you go.\n\n" +
		"### API\n```go\npackage solution\n\nfunc F(x int) int { panic(\"not implemented\") }\n```\n\n" +
		"### VISIBLE TESTS\n```go\npackage solution\n\nimport \"testing\"\n\nfunc TestV_A(t *testing.T) {}\n```\n\n" +
		"### HIDDEN TESTS\n```go\npackage solution\n\nimport \"testing\"\n\nfunc TestH_A(t *testing.T) {}\n```\n"
	s, err := ParseSpec(reply)
	if err != nil {
		t.Fatal(err)
	}
	if s.API == "" || s.VisibleTests == "" || s.HiddenTests == "" {
		t.Fatalf("empty block in %+v", s)
	}
	// The partitions must not leak into one another: the hidden tests are the
	// acceptance oracle and must never reach a repair prompt.
	if got := s.VisibleTests; contains(got, "TestH_") {
		t.Error("hidden tests leaked into the visible partition")
	}
	if got := s.HiddenTests; contains(got, "TestV_") {
		t.Error("visible tests leaked into the hidden partition")
	}
}

func TestParseSpecRejectsIncomplete(t *testing.T) {
	cases := map[string]string{
		"missing a heading": "### API\n```go\npackage solution\n```\n### VISIBLE TESTS\n```go\nx\n```\n",
		"heading with no code block": "### API\nnothing\n\n### VISIBLE TESTS\n```go\nx\n```\n\n" +
			"### HIDDEN TESTS\n```go\ny\n```\n",
		"empty": "",
	}
	for name, reply := range cases {
		if _, err := ParseSpec(reply); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

func TestExtractCode(t *testing.T) {
	if got, err := ExtractCode("prose\n```go\npackage solution\n```\ntrailing"); err != nil ||
		got != "package solution" {
		t.Errorf("fenced: got %q err %v", got, err)
	}
	if got, err := ExtractCode("```\npackage solution\n```"); err != nil || got != "package solution" {
		t.Errorf("unlabelled fence: got %q err %v", got, err)
	}
	// Some models drop the fence entirely; accept that rather than failing a
	// perfectly good candidate on formatting.
	if got, err := ExtractCode("package solution\n\nfunc F() {}"); err != nil ||
		!contains(got, "package solution") {
		t.Errorf("unfenced: got %q err %v", got, err)
	}
	if _, err := ExtractCode("I cannot help with that."); err == nil {
		t.Error("expected an error when there is no code at all")
	}
}

// The repair prompt must carry the diagnostic and the previous attempt, and
// must never carry the held-out tests.
func TestRepairUserCarriesDiagnosticNotHiddenTests(t *testing.T) {
	out := RepairUser("problem", "func F()", "package solution", "V1:types", "undefined: slices.MaxRunFunc")
	for _, want := range []string{"problem", "func F()", "package solution", "V1:types", "MaxRunFunc"} {
		if !contains(out, want) {
			t.Errorf("repair prompt is missing %q", want)
		}
	}
	if contains(out, "TestH_") {
		t.Error("repair prompt contains held-out tests")
	}
}

func TestCodeUserIncludesNegativeConstraints(t *testing.T) {
	out := CodeUser("problem", "func F()", 3, []string{"V1:types: undefined: slices.MaxRunFunc"})
	if !contains(out, "MaxRunFunc") {
		t.Error("refuted approaches were not fed forward as negative constraints")
	}
	if !contains(out, "sample 3") {
		t.Error("the sample nonce is missing, so parallel samples would be identical")
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && indexOf(hay, needle) >= 0
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
