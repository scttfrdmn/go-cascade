// Package prompt builds the model-facing prompts and parses their replies.
//
// The pipeline is deliberately two-phase: a spec model derives the API contract
// and the tests *before* any solution exists, and the code models then write
// against that fixed contract. This is what makes the oracle independent of the
// code author and what makes cached solutions checkable against a new query.
package prompt

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
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
- Deterministic only: no time-of-day, no unseeded randomness, no network.

Correctness of expected values is critical. A test that asserts the wrong
answer rejects every correct implementation, which is worse than no test. For
each test:
- Compute the expected value by hand, stepping through the input, before you
  write the assertion. Do not guess it from the input's shape.
- Re-scan the whole input for the property in question. A contiguous run,
  match, or extreme can begin one element earlier or later than it first
  appears; an adjacent element often extends the case you had in mind.
- If a comment states the reasoning, make the asserted value agree with the
  comment. If they disagree, recompute both.
When unsure of an expected value, omit that test rather than assert a value you
have not verified.`

// SpecUser renders the spec-phase user turn.
func SpecUser(problem string) string {
	return "Problem statement:\n\n" + strings.TrimSpace(problem)
}

// SpecSystem returns the spec-phase system prompt.
func SpecSystem() string { return specSystem }

// specSystemPinned is the spec prompt used when the API is *given* rather than
// invented. The only difference from specSystem is the API section: the model
// must reproduce the provided contract verbatim instead of deriving its own, so
// the generated tests are written against exactly the signatures a known-correct
// reference implements. This is what lets the reference-validation gate
// (calibrate -refs) reach a verdict instead of failing to compile on a name
// mismatch. Everything about test quality is held identical to specSystem so the
// pinned/unpinned comparison isolates the API-agreement variable alone.
const specSystemPinned = `You write Go test suites. You never write the implementation.

You are GIVEN the exact API the implementation must provide. Do not change it,
rename anything, or add or remove parameters or results. Respond with exactly
three fenced Go blocks under these headings, in this order and with no other
prose:

### API
Reproduce the provided API block verbatim as a single fenced go block: package
solution, the given exported signatures, each function body exactly
"panic(\"not implemented\")". Do not alter names or signatures.

### VISIBLE TESTS
A single fenced go block: package solution, importing "testing". Every test
function name MUST begin with TestV. These cover the stated behaviour and the
obvious edge cases. Call only the functions named in the API.

### HIDDEN TESTS
A single fenced go block: package solution, importing "testing". Every test
function name MUST begin with TestH. These are adversarial: boundary values,
empty and nil inputs, large inputs, aliasing, and any concurrency hazard implied
by the problem. They must not duplicate the visible tests. Call only the
functions named in the API.

Rules:
- Standard library only. No third-party imports.
- Tests must compile against the API block alone, using the given names exactly.
- Do not use testing.T.Parallel unless the problem is about concurrency.
- Deterministic only: no time-of-day, no unseeded randomness, no network.

Correctness of expected values is critical. A test that asserts the wrong
answer rejects every correct implementation, which is worse than no test. For
each test:
- Compute the expected value by hand, stepping through the input, before you
  write the assertion. Do not guess it from the input's shape.
- Re-scan the whole input for the property in question. A contiguous run,
  match, or extreme can begin one element earlier or later than it first
  appears; an adjacent element often extends the case you had in mind.
- If a comment states the reasoning, make the asserted value agree with the
  comment. If they disagree, recompute both.
When unsure of an expected value, omit that test rather than assert a value you
have not verified.`

// SpecSystemPinned returns the spec-phase system prompt for the pinned-API mode.
func SpecSystemPinned() string { return specSystemPinned }

// SpecUserPinned renders the spec-phase user turn when the API is pinned: the
// problem statement plus the exact contract the tests must be written against.
func SpecUserPinned(problem, api string) string {
	var b strings.Builder
	b.WriteString("Problem statement:\n\n")
	b.WriteString(strings.TrimSpace(problem))
	b.WriteString("\n\nUse exactly this API (do not rename or change signatures):\n\n```go\n")
	b.WriteString(strings.TrimSpace(api))
	b.WriteString("\n```")
	return b.String()
}

// ExtractAPI reduces a complete Go source file to an "API block": the exported
// type declarations and exported function/method signatures, with every function
// body replaced by panic("not implemented"). It is how a validated reference
// solution is turned into the pinned contract fed to the spec model, so the
// pinned API is authoritative (it is exactly what the reference compiles as)
// rather than hand-transcribed. Unexported declarations are dropped; imports are
// preserved because a signature may reference an imported type (e.g. context).
func ExtractAPI(src string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse reference: %w", err)
	}
	var imports, keep []ast.Decl
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				imports = append(imports, decl)
				continue
			}
			// Keep exported type declarations (structs, interfaces, aliases); a
			// method set or a public signature can depend on them.
			if decl.Tok == token.TYPE {
				keep = append(keep, decl)
			}
		case *ast.FuncDecl:
			// Keep exported functions and every method on an exported receiver
			// type (a method's own name may be unexported but still part of the
			// public surface, e.g. a String() on a public type).
			if decl.Name.IsExported() || decl.Recv != nil {
				decl.Body = &ast.BlockStmt{List: []ast.Stmt{
					&ast.ExprStmt{X: &ast.CallExpr{
						Fun:  ast.NewIdent("panic"),
						Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"not implemented"`}},
					}},
				}}
				keep = append(keep, decl)
			}
		}
	}
	f.Decls = append(imports, keep...)
	// Drop file-level doc so the block starts at the package clause, and clear the
	// file's comment list: the printer would otherwise re-emit orphaned comments
	// from dropped (unexported) declarations as floating text. Each kept decl still
	// carries its own Doc field, which prints attached.
	f.Doc = nil
	f.Comments = nil
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, f); err != nil {
		return "", fmt.Errorf("print API: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

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

const planSystem = `You are a senior Go engineer writing an implementation plan for
another engineer to code from. You do NOT write the implementation yourself.

Given a problem statement and the exact API to implement, respond with a concise
plan in prose and short pseudo-steps: the algorithm and its complexity, the data
structures, the edge cases that must be handled (empty/nil input, boundaries,
overflow, aliasing, Unicode, concurrency hazards), and any subtle correctness
traps in the spec. Be specific about anything easy to get wrong.

Rules:
- Do NOT write Go implementation code or a full function body. Pseudo-code steps
  and signatures are fine; a finished solution is not.
- Standard library only.
- Do not restate the problem; add the reasoning a careful implementer needs.`

// PlanSystem returns the planning-phase system prompt for a two-stage tier.
func PlanSystem() string { return planSystem }

// PlanUser renders the planning-phase user turn: the problem plus the fixed API
// the coder must implement. The planner sees only what the coder already sees
// (problem + API); it must NOT be shown the hidden tests (invariant #1/#2).
func PlanUser(problem, api string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Problem statement:\n\n%s\n\n", strings.TrimSpace(problem))
	fmt.Fprintf(&b, "The implementation must satisfy exactly this API:\n\n```go\n%s\n```\n",
		strings.TrimSpace(api))
	return b.String()
}

// CodeUserFromPlan renders the code-phase user turn for a two-stage tier: the
// same contract as CodeUser plus an implementation plan authored by the planner
// model. The plan is advisory context, not a spec — the API remains
// authoritative — so a coder that disagrees with the plan still must satisfy the
// API and will be caught by the same oracle.
func CodeUserFromPlan(problem, api, plan string, nonce int, avoid []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Problem statement:\n\n%s\n\n", strings.TrimSpace(problem))
	fmt.Fprintf(&b, "Implement exactly this API:\n\n```go\n%s\n```\n", strings.TrimSpace(api))
	if p := strings.TrimSpace(plan); p != "" {
		fmt.Fprintf(&b, "\nAn implementation plan from a senior engineer (advisory; the API above is authoritative):\n\n%s\n", p)
	}
	if len(avoid) > 0 {
		b.WriteString("\nThese approaches were already tried and refuted; do not repeat them:\n")
		for _, a := range avoid {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	fmt.Fprintf(&b, "\n(sample %d)\n", nonce)
	return b.String()
}

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

// JudgeStrictness sets where the judge's PASS/FAIL boundary sits when it is
// uncertain. It is the knob that traces the judge oracle's η_fa/β operating
// curve: a permissive judge accepts on doubt (raising false acceptances), a
// strict judge rejects on doubt (raising false rejections). Only the tie-break
// instruction changes; everything else about the prompt is held fixed so the
// sweep isolates the operating point.
type JudgeStrictness string

// Judge strictness levels.
const (
	JudgeStrict     JudgeStrictness = "strict"     // when in doubt, FAIL
	JudgeBalanced   JudgeStrictness = "balanced"   // no thumb on the scale
	JudgePermissive JudgeStrictness = "permissive" // when in doubt, PASS
)

const judgeSystemBase = `You are a code reviewer. You are given a Go problem
statement, the API the solution must satisfy, and a candidate implementation.
Decide whether the implementation is correct for every valid input, not merely
plausible.

You cannot run the code. Judge by reading alone.

Respond with exactly one line, nothing else:
VERDICT: PASS
or
VERDICT: FAIL

PASS means the code is correct for all valid inputs, including boundary and
adversarial ones. FAIL means you found, or suspect, a defect. `

var judgeTieBreak = map[JudgeStrictness]string{
	JudgeStrict:     "When in doubt, FAIL.",
	JudgeBalanced:   "Weigh the evidence and give your best single judgement; do not default either way.",
	JudgePermissive: "When in doubt, PASS.",
}

// JudgeSystem returns the judge-phase system prompt at the given strictness. An
// unrecognised value falls back to strict, which is the conservative default and
// the behaviour prior to the strictness knob.
func JudgeSystem(s JudgeStrictness) string {
	tie, ok := judgeTieBreak[s]
	if !ok {
		tie = judgeTieBreak[JudgeStrict]
	}
	return judgeSystemBase + tie
}

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
