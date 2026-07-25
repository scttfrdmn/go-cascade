// Package prompt builds the model-facing prompts and parses their replies.
//
// The pipeline is deliberately two-phase: a spec model derives the API contract
// and the tests *before* any solution exists, and the code models then write
// against that fixed contract. This is what makes the oracle independent of the
// code author and what makes cached solutions checkable against a new query.
package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

// Spec is the contract produced by the spec phase.
type Spec struct {
	API          string // the exported signatures the solution must provide
	VisibleTests string // used by the repair loop
	HiddenTests  string // held out for acceptance; never shown to a code model
}

const specSystem = `You write Go test suites. You never write the implementation.

Given a problem statement, respond with exactly three fenced Go blocks under
these headings, in this order and with no other prose:

### API
A single fenced go block: package solution, containing ONLY the exported
function/type signatures the implementation must provide, with a doc comment on
each. No bodies (use a bare declaration with no body is invalid Go, so give each
function a body of exactly "panic(\"not implemented\")").

### VISIBLE TESTS
A single fenced go block: package solution, importing "testing". Every test
function name MUST begin with TestV. These cover the stated behaviour and the
obvious edge cases.

### HIDDEN TESTS
A single fenced go block: package solution, importing "testing". Every test
function name MUST begin with TestH. These are adversarial: boundary values,
empty and nil inputs, large inputs, aliasing, and any concurrency hazard implied
by the problem. They must not duplicate the visible tests.

Rules:
- Standard library only. No third-party imports.
- Tests must compile against the API block alone.
- Do not use testing.T.Parallel unless the problem is about concurrency.
- Deterministic only: no time-of-day, no unseeded randomness, no network.`

// SpecUser renders the spec-phase user turn.
func SpecUser(problem string) string {
	return "Problem statement:\n\n" + strings.TrimSpace(problem)
}

// SpecSystem returns the spec-phase system prompt.
func SpecSystem() string { return specSystem }

const codeSystem = `You write Go implementations. You are given a fixed API and
you must satisfy it exactly.

Respond with exactly one fenced go block and no other prose. The block is a
complete single file: package solution, standard library only, no func main,
no TODO comments, no panics for unimplemented paths.

Correctness first, then allocations, then clarity. Idiomatic Go: handle errors,
respect context cancellation if a context is in the signature, and do not leak
goroutines.`

// CodeUser renders the code-phase user turn.
func CodeUser(problem, api string, nonce int, avoid []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Problem statement:\n\n%s\n\n", strings.TrimSpace(problem))
	fmt.Fprintf(&b, "Implement exactly this API:\n\n```go\n%s\n```\n", strings.TrimSpace(api))
	if len(avoid) > 0 {
		b.WriteString("\nThese approaches were already tried and refuted; do not repeat them:\n")
		for _, a := range avoid {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	// Bedrock exposes no seed for Claude, so sample diversity comes from
	// temperature plus this nonce.
	fmt.Fprintf(&b, "\n(sample %d)\n", nonce)
	return b.String()
}

// CodeSystem returns the code-phase system prompt.
func CodeSystem() string { return codeSystem }

// RepairUser renders a repair turn: the previous attempt plus the exact
// verifier diagnostics that refuted it.
func RepairUser(problem, api, prev, stage, diag string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Problem statement:\n\n%s\n\n", strings.TrimSpace(problem))
	fmt.Fprintf(&b, "Required API:\n\n```go\n%s\n```\n\n", strings.TrimSpace(api))
	fmt.Fprintf(&b, "Your previous attempt:\n\n```go\n%s\n```\n\n", strings.TrimSpace(prev))
	fmt.Fprintf(&b, "It was refuted at verifier stage %s. Exact output:\n\n```\n%s\n```\n\n",
		stage, strings.TrimSpace(diag))
	b.WriteString("Return the corrected complete file as one fenced go block. " +
		"Fix the reported defect; do not restructure working code.")
	return b.String()
}

const judgeSystem = `You are a strict code reviewer. You are given a Go problem
statement, the API the solution must satisfy, and a candidate implementation.
Decide whether the implementation is correct for every valid input, not merely
plausible.

You cannot run the code. Judge by reading alone.

Respond with exactly one line, nothing else:
VERDICT: PASS
or
VERDICT: FAIL

PASS means you are confident the code is correct for all valid inputs, including
boundary and adversarial ones. FAIL means you found, or strongly suspect, a
defect. When in doubt, FAIL.`

// JudgeSystem returns the judge-phase system prompt.
func JudgeSystem() string { return judgeSystem }

// JudgeUser renders a judge turn. It deliberately withholds the test suites:
// the judge is the alternative to an executable oracle, so giving it the tests
// would turn it back into one and defeat the comparison (paper §5.5c).
func JudgeUser(problem, api, candidate string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Problem statement:\n\n%s\n\n", strings.TrimSpace(problem))
	fmt.Fprintf(&b, "Required API:\n\n```go\n%s\n```\n\n", strings.TrimSpace(api))
	fmt.Fprintf(&b, "Candidate implementation:\n\n```go\n%s\n```\n", strings.TrimSpace(candidate))
	return b.String()
}

var judgeRE = regexp.MustCompile(`(?i)VERDICT:\s*(PASS|FAIL)`)

// ParseJudge extracts a PASS/FAIL verdict. A reply with no parseable verdict is
// treated by the caller as a refusal to pass; ParseJudge reports that as an
// error so the caller can decide the safe default rather than guessing here.
func ParseJudge(reply string) (pass bool, err error) {
	m := judgeRE.FindStringSubmatch(reply)
	if m == nil {
		return false, fmt.Errorf("no VERDICT: PASS/FAIL line in judge reply (%d bytes)", len(reply))
	}
	return strings.EqualFold(m[1], "PASS"), nil
}

var (
	fenceRE   = regexp.MustCompile("(?s)```(?:go|golang)?\\s*\\n(.*?)```")
	headingRE = regexp.MustCompile(`(?mi)^#{1,6}\s*(API|VISIBLE TESTS|HIDDEN TESTS)\s*$`)
)

// ExtractCode returns the first fenced code block, or the whole reply if the
// model omitted fences and the text parses as a Go file.
func ExtractCode(reply string) (string, error) {
	if m := fenceRE.FindStringSubmatch(reply); m != nil {
		return strings.TrimSpace(m[1]), nil
	}
	if strings.Contains(reply, "package ") {
		return strings.TrimSpace(reply), nil
	}
	return "", fmt.Errorf("no go code block in reply (%d bytes)", len(reply))
}

// ParseSpec splits a spec-phase reply into its three blocks.
func ParseSpec(reply string) (*Spec, error) {
	locs := headingRE.FindAllStringSubmatchIndex(reply, -1)
	if len(locs) < 3 {
		return nil, fmt.Errorf("spec reply has %d of 3 required headings", len(locs))
	}
	s := &Spec{}
	for i, loc := range locs {
		name := strings.ToUpper(reply[loc[2]:loc[3]])
		end := len(reply)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := reply[loc[1]:end]
		m := fenceRE.FindStringSubmatch(body)
		if m == nil {
			return nil, fmt.Errorf("no code block under heading %q", name)
		}
		code := strings.TrimSpace(m[1])
		switch name {
		case "API":
			s.API = code
		case "VISIBLE TESTS":
			s.VisibleTests = code
		case "HIDDEN TESTS":
			s.HiddenTests = code
		}
	}
	if s.API == "" || s.VisibleTests == "" || s.HiddenTests == "" {
		return nil, fmt.Errorf("spec reply is missing one of API/visible/hidden")
	}
	return s, nil
}
