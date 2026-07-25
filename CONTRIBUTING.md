# Contributing

## Before you start

Read `docs/verification-saturated-cascades.md`, at least §2.5 (the risk
certificate), §3.1 (why the oracle must be sound), and §5 (what this project
does and does not establish).

Then read the **Invariants** section of `CLAUDE.md`. Several of them can be
violated without any test failing, and each one silently invalidates the
certificate the tool prints.

## Ground rules

- `make check` must pass: `gofmt`, `go vet`, `golangci-lint`, full test suite.
- Comments explain *why*. Where a line encodes a statistical requirement, say so
  — those comments are the documentation that stops the next person "simplifying"
  a load-bearing constraint.
- Tests assert mechanism, not just outcome. Ladder tests assert *which stage*
  refutes each defect class; calibration tests check against closed forms.

## Never cite a mock number as a measurement

`internal/model/mock.go` has a defect distribution that was stipulated by hand to
exercise code paths. Numbers it produces describe the mock. They are not
measurements of any language model, and the README and paper are careful about
this. Please keep them careful.

## High-value contributions

Ranked roughly by how much they would improve the project's standing:

1. **A real evaluation.** §5.5 of the paper specifies it: 400+ Go problems, five
   arms including a judge-oracle variant, primary outcome being the lowest alpha
   each arm can certify at fixed delta and n. Without this the central
   comparative claim stays argued rather than demonstrated. The judge-oracle arm
   itself is now implemented (`calibrate --oracle=judge`, `--compare`; see
   `internal/cascade/judge.go`) and demonstrated against the mock, so what
   remains is the live run on a real benchmark — which is gated on building that
   benchmark, since `examples/problems.jsonl` is 18 variants of one task.
2. **Oracle-gap validation.** Correlate mutation score against measured
   false-acceptance rate on problems with reference implementations. This tests
   whether §3.7's estimator is usable at all; its bias direction is currently
   unknown.
3. **Adaptive conformal inference** (paper eq. 9) layered on the static bound,
   for drift tracking.
4. **Group-conditional (Mondrian) conformal** so concurrency-heavy tasks get
   their own risk group, rather than hiding inside a marginal average.
5. **Non-nested tier ordering** via Weitzman's index, conditioned on a coarse
   query cluster.
6. **Multi-file and third-party-dependency support**, which removes the
   stdlib-only restriction and changes the verifier economics.

## Adding a verifier stage

A new stage must be **sound**: failure implies incorrectness. If it can reject a
correct program — a flaky check, a timeout, a heuristic, anything model-based —
it is not a ladder stage. Model it as a separate risk term with its own alpha, or
leave it out. Adding an unsound stage to the ladder breaks the argument that
verification reduces cost at fixed risk, which is the reason this design is worth
anything.
