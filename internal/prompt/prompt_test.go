package prompt

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestExtractAPIBlanksBodiesKeepsSignatures(t *testing.T) {
	src := `package solution

import "context"

// Fibonacci returns the nth Fibonacci number.
func Fibonacci(n int) uint64 {
	var a, b uint64 = 0, 1
	for range n {
		a, b = b, a+b
	}
	return a
}

// helper is unexported and must be dropped.
func helper() int { return 42 }
`
	api, err := ExtractAPI(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(api, "func Fibonacci(n int) uint64") {
		t.Errorf("exported signature missing:\n%s", api)
	}
	if !strings.Contains(api, `panic("not implemented")`) {
		t.Errorf("body was not blanked to panic:\n%s", api)
	}
	if strings.Contains(api, "a, b = b, a+b") {
		t.Errorf("original body leaked into the API block:\n%s", api)
	}
	if strings.Contains(api, "helper") {
		t.Errorf("unexported func must be dropped:\n%s", api)
	}
	// A kept declaration's own doc comment must survive — it is useful context
	// for the spec model — even though orphaned comments of dropped decls are cut.
	if !strings.Contains(api, "Fibonacci returns the nth") {
		t.Errorf("kept declaration lost its doc comment:\n%s", api)
	}
	// "context" must NOT survive: nothing that remains refers to it, and in Go an
	// unused import is a compile error. This assertion was previously inverted —
	// it required the import to be kept, which is what made ExtractAPI emit
	// non-compiling API blocks for 28.3% of the MultiPL-E benchmark. Whether an
	// import is *needed* is the question, not whether one existed.
	if strings.Contains(api, `"context"`) {
		t.Errorf("unused import must be pruned or the API block will not compile:\n%s", api)
	}
}

// The API block ExtractAPI emits must COMPILE, which is a stronger property than
// any assertion about its text and is the one that actually matters: the block is
// pasted into the spec prompt under "use exactly this API", so a block that does
// not compile propagates into spec.API, breaks testsCompileAgainstOwnAPI, and
// makes validateOracle flag a sound oracle as OracleUnsound (invariant #4).
//
// Type-checked with go/types rather than by shelling out to the toolchain, so it
// runs under -short like the rest of this package.
func TestExtractAPICompiles(t *testing.T) {
	cases := []struct {
		name, src string
		wantKept  []string // imports that must survive because something still uses them
		wantGone  []string // imports whose only use was in a body
	}{{
		name: "import used only by a body",
		src: `package solution

import (
	"math"
	"strings"
)

func Area(r float64) float64 { return math.Pi * r * r }

func Shout(s string) string { return strings.ToUpper(s) }
`,
		wantGone: []string{"math", "strings"},
	}, {
		name: "import used by a signature",
		src: `package solution

import (
	"context"
	"strings"
)

func Fetch(ctx context.Context, url string) (string, error) {
	return strings.TrimSpace(url), nil
}
`,
		wantKept: []string{"context"}, // parameter type
		wantGone: []string{"strings"}, // body only
	}, {
		name: "import used by a surviving struct field",
		src: `package solution

import (
	"sync"
	"time"
)

type Limiter struct {
	mu sync.Mutex
}

func (l *Limiter) Wait(d time.Duration) { time.Sleep(d) }
`,
		wantKept: []string{"sync", "time"}, // field type; parameter type
	}, {
		name: "aliased import used by a signature",
		src: `package solution

import (
	ttime "time"
	"unicode/utf8"
)

func Deadline() ttime.Duration { return 0 }

func Runes(s string) int { return utf8.RuneCountInString(s) }
`,
		// The alias is referenced by a signature; the other only by a body. Pins
		// that pruning keys off the LOCAL name (ttime) rather than the path's last
		// element (time) — a name-based check that ignored the alias would drop it.
		wantKept: []string{`ttime "time"`},
		wantGone: []string{"utf8"},
	}, {
		name: "blank and dot imports are never dropped",
		src: `package solution

import (
	_ "embed"
	"math"
)

func Zero() int { return int(math.Abs(0)) }
`,
		// A blank import exists for its side effect and has no textual use, so it
		// cannot be tracked by name and must be kept.
		wantKept: []string{"embed"},
		wantGone: []string{"math"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api, err := ExtractAPI(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.wantKept {
				if !strings.Contains(api, w) {
					t.Errorf("needed import %q was pruned:\n%s", w, api)
				}
			}
			for _, w := range tc.wantGone {
				if strings.Contains(api, w) {
					t.Errorf("unused import %q survived:\n%s", w, api)
				}
			}
			mustTypeCheck(t, api)
		})
	}
}

// mustTypeCheck fails unless src is a well-formed, fully type-correct package.
// An unused import is reported here exactly as the compiler would report it,
// which is the whole point: that is the error class ExtractAPI used to emit.
func mustTypeCheck(t *testing.T, src string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, 0)
	if err != nil {
		t.Fatalf("emitted API block does not parse: %v\n%s", err, src)
	}
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("solution", fset, []*ast.File{f}, nil); err != nil {
		t.Errorf("emitted API block does not type-check: %v\n%s", err, src)
	}
}

// A struct type plus its methods (an exported receiver) must be preserved in
// full, because the tests are written against that public surface.
func TestExtractAPIKeepsTypesAndMethods(t *testing.T) {
	src := `package solution

import "sync"

// RateLimiter is a token bucket.
type RateLimiter struct {
	mu     sync.Mutex
	tokens int64
}

// NewRateLimiter builds one.
func NewRateLimiter(capacity int64) *RateLimiter {
	return &RateLimiter{tokens: capacity}
}

// Allow consumes n tokens.
func (r *RateLimiter) Allow(n int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return true
}
`
	api, err := ExtractAPI(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type RateLimiter struct", "func NewRateLimiter(capacity int64) *RateLimiter", "func (r *RateLimiter) Allow(n int64) bool"} {
		if !strings.Contains(api, want) {
			t.Errorf("API block missing %q:\n%s", want, api)
		}
	}
	if strings.Contains(api, "r.mu.Lock()") {
		t.Errorf("method body leaked:\n%s", api)
	}
}

// The pinned spec user prompt must carry the API verbatim so the model writes
// tests against exactly those names.
func TestSpecUserPinnedIncludesAPI(t *testing.T) {
	api := "package solution\n\nfunc TwoSum(nums []int, target int) [2]int { panic(\"not implemented\") }"
	got := SpecUserPinned("Return the indices...", api)
	if !strings.Contains(got, "func TwoSum(nums []int, target int) [2]int") {
		t.Errorf("pinned user prompt dropped the API:\n%s", got)
	}
	if !strings.Contains(got, "do not rename") {
		t.Errorf("pinned user prompt should instruct against renaming:\n%s", got)
	}
}

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

func TestParseJudge(t *testing.T) {
	cases := []struct {
		reply    string
		wantPass bool
		wantErr  bool
	}{
		{"VERDICT: PASS", true, false},
		{"VERDICT: FAIL", false, false},
		{"Some reasoning.\nVERDICT: pass\n", true, false}, // case-insensitive
		{"VERDICT:FAIL", false, false},                    // no space
		{"I think this is fine.", false, true},            // no verdict line => error
		{"", false, true},
	}
	for _, c := range cases {
		pass, err := ParseJudge(c.reply)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseJudge(%q): err=%v, wantErr=%v", c.reply, err, c.wantErr)
		}
		if err == nil && pass != c.wantPass {
			t.Errorf("ParseJudge(%q): pass=%v, want %v", c.reply, pass, c.wantPass)
		}
	}
}

func TestJudgeSystemStrictness(t *testing.T) {
	// Each level must carry its own tie-break instruction and share the rest.
	strict := JudgeSystem(JudgeStrict)
	perm := JudgeSystem(JudgePermissive)
	bal := JudgeSystem(JudgeBalanced)
	if !contains(strict, "When in doubt, FAIL") {
		t.Errorf("strict prompt missing FAIL tie-break:\n%s", strict)
	}
	if !contains(perm, "When in doubt, PASS") {
		t.Errorf("permissive prompt missing PASS tie-break:\n%s", perm)
	}
	if contains(bal, "When in doubt") {
		t.Errorf("balanced prompt should not default either way:\n%s", bal)
	}
	// Unknown strictness falls back to strict (the conservative default).
	if JudgeSystem("bogus") != strict {
		t.Error("unknown strictness should fall back to strict")
	}
	// The base (everything but the tie-break) must be identical across levels,
	// so the sweep isolates the operating point and nothing else.
	trim := func(s, suffix string) string { return s[:len(s)-len(suffix)] }
	if trim(strict, "When in doubt, FAIL.") != trim(perm, "When in doubt, PASS.") {
		t.Error("strict and permissive prompts differ beyond the tie-break line")
	}
}

func TestJudgeUserWithholdsTests(t *testing.T) {
	// The judge is the alternative to an executable oracle. If the prompt leaked
	// the test suites it would become one, defeating the comparison (§5.5c).
	u := JudgeUser("do a thing", "package solution\n\nfunc F() {}", "package solution\n\nfunc F() {}")
	if contains(u, "TestV") || contains(u, "TestH") || contains(u, "testing") {
		t.Errorf("judge prompt leaked test material:\n%s", u)
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
