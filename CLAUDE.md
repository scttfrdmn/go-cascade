# CLAUDE.md

Project instructions for Claude Code working in this repository.

## What this is

`go-cascade` routes a Go coding problem to the cheapest language model that
provably solves it. Candidates are verified by **execution**, not by a judge
model, and thresholds carry a distribution-free finite-sample risk certificate.

The design rationale, proofs, and an honest account of what is and is not
validated live in `docs/verification-saturated-cascades.md`. **Read it before
changing anything in `internal/calibrate`, `internal/cluster`, or the acceptance
path in `internal/cascade`.** Those three carry load-bearing statistical claims
that are easy to break without any test failing.

## Commands

```bash
make build          # build ./bin/go-cascade
make test           # full suite (compiles and runs generated code; ~1 min)
make test-short     # skip the tests that shell out to the toolchain
make check          # gofmt -l, go vet, golangci-lint, test
make demo           # end-to-end run against the mock provider, no credentials
```

Run a single package: `go test ./internal/verify/ -run TestLadderRefutations -v`

## Requirements

- Go 1.26+ (uses `for range int`, `slices.SortFunc`, `strings.SplitSeq`)
- A working `go` toolchain **on PATH at runtime** — the verifier shells out to
  it. This is not a build-time-only dependency.
- AWS credentials with Bedrock access, *only* for `--provider=bedrock`.
  Everything else works against `--provider=mock`.

## Architecture

```
cmd/go-cascade       CLI: solve, calibrate, models, cache
internal/cascade     router: arm zero, sampling, repair, escalation, speculation
internal/verify      verifier ladder, ephemeral workspaces, mutation analysis
internal/cluster     behavioural clustering, Wilson lower bound
internal/cache       arm zero: solutions, specs, failures
internal/calibrate   Learn-then-Test, Hoeffding-Bentkus, certificates
internal/model       Bedrock Converse provider; deterministic mock
internal/prompt      two-phase prompting and reply parsing
internal/config      tiers, cost model, risk knobs
```

Request flow: `spec` (generate contract + tests) → `cache` (arm zero, verified
transfer) → tier loop (sample → verify → cluster → accept/repair/escalate) →
`accept` (held-out partition) → mutation analysis → cache admission.

## Invariants — do not break these

These are correctness properties of the *method*, not style preferences. Each has
a test, but the tests will not catch every way of violating them.

1. **Never repair against the hidden partition.** `TestH*` is the acceptance
   oracle. The repair loop sees `TestV*` only. Feeding hidden-test diagnostics
   into a repair prompt destroys the holdout and silently invalidates every
   certificate.

2. **Never shop against the holdout.** When a candidate fails acceptance, the
   router *escalates*. Do not try the next candidate from the same tier until one
   passes — that is selection against the holdout and inflates true risk above
   the certified bound. This costs money on purpose.

3. **The test author must differ from the code author.** If a tier's `ModelID`
   equals `cfg.TestModel`, the run is flagged `OracleContaminated` and excluded
   from calibration. Do not "fix" a failing calibration by relaxing this.

4. **A failed verifier stage is a sound refutation.** `OK == false` means
   incorrect, full stop — no probability, no threshold, no contribution to the
   risk budget. Any change that makes a stage probabilistic (a flaky check, a
   timeout treated as a pass, an LLM-based stage) breaks the central argument
   that verification reduces cost at fixed risk. If you add a stage, it must be
   sound or it must be modelled as a separate risk term. *Corollary:* the oracle
   itself must be sound — a **generated** test suite that refutes correct code is
   an unsound oracle and its labels are noise. `calibrate -refs <dir>` runs each
   problem's execution-validated reference through the generated tests; a refuted
   reference flags the record `OracleUnsound` and excludes it from calibration
   (`Router.validateOracle`, mirrored on the `Contaminated` exclusion). Without
   `-refs` the spec model's test bugs silently inflate the measured risk — this
   is not hypothetical, it was observed live (spec model asserting a wrong
   expected value / a mismatched function name on trivial problems).

5. **Cache hits are verified, never predicted.** A retrieved solution is
   re-executed against the *new* query's tests. Do not add a similarity
   threshold that returns a cached answer without running it. Retrieval quality
   affects only how often the cache is worth consulting.

6. **Never claim certification without a valid certificate.** `Result.Certified`
   comes from `Router.calibrated()`, which requires a loaded certificate with
   `Valid == true`. Uncalibrated runs must print `UNCERTIFIED`. The final tier
   has no threshold — that is by construction and is *not* the same as being
   uncalibrated.

7. **Fixed-sequence ordering in LTT must be data-independent.** `Calibrate`
   orders the grid by descending threshold magnitude. Ordering by observed risk
   or cost is data-dependent and voids FWER control. See §2.5 of the paper.

8. **Calibrate on the cache-bypass stream.** `Profile` sets `Shadow: true` and
   `cmdCalibrate` forces `cfg.CacheDir = ""`. A warm cache absorbs the head of
   the query distribution, so calibrating behind it breaks exchangeability. See
   §2.9.

9. **The routing score must be monotone in evidence.** `cluster.Score` returns a
   Wilson lower bound, not the raw cluster mass. Reverting to raw mass
   reintroduces the failure documented in §4.3: a two-sample tier reports 1.0
   unconditionally and no threshold can certify.

## Gotchas discovered the hard way

- **`go build` does not compile `_test.go` files.** A solution/oracle signature
  mismatch survives the build stage and is caught by `go vet`. That is why vet is
  in the ladder at all.
- **`go vet` (113 ms) costs more than `go build` (43 ms)**, so build runs first.
  This is the reverse of the conventional pipeline order and is deliberate.
- **`GOCACHE` must persist across processes.** A per-run cache costs ~30 s of
  cold compilation on the first candidate. `verify.NewRunner` puts it under
  `os.UserCacheDir()`. Do not move it back into the scratch root.
- **The `go/types` source importer must be shared and warmed.** Cold it is
  237 ms; warm it is ~1 ms. `verify.NewLadder` creates one per router and
  `Warm()` preloads common stdlib packages. It is not goroutine-safe, hence the
  mutex.
- **The race stage is 32× a plain test run.** It is gated on an AST predicate.
  Do not ungate it.
- **Bedrock model IDs churn.** They are configuration, never constants. Use
  `go-cascade models` (calls `ListInferenceProfiles`) to discover real ones.
- **Long `calibrate` runs checkpoint and resume.** A paired run over n=64 takes
  ~60–90 min and can be SIGTERM'd externally (issue #21). The loop writes records
  after *every* problem, treats a cancelled context as a clean stop (not a
  per-problem "skip"), and prints an INTERRUPTED hint. Re-run the identical command
  with `-resume` to skip already-recorded ids and finish the rest. Do not revert to
  writing records only at the end — a kill then discards the whole run.

## Testing conventions

- Tests that shell out to the toolchain guard with `testing.Short()`.
- `verify` tests assert *which stage* refutes each defect class, not merely that
  it is refuted. Keep that — it is how ladder-ordering regressions surface.
- `calibrate` tests check the statistics against closed forms (exact binomial
  summation, $(1-\alpha)^n$ at zero errors). Do not replace with golden values.
- The mock provider's defect distribution is stipulated, not sampled. It exists
  to exercise code paths. **Never cite a number produced by the mock as a
  measurement of model behaviour.**

## Known gaps (contributions welcome)

- **Live evaluation has been run** (17 experiments against Bedrock; see `results/`
  and paper §5.6). It is **demonstrated, not validated**: n ≤ 64, far below the
  §5.5 bar of n ≥ 300, on a small non-standard single-file/stdlib benchmark. The
  full §5.5 experiment (n ≥ 300, standard benchmark, all five arms) remains unrun —
  that is the open gap, not "no live run." Of the two secondary tests, §5.5(5) (the
  §3.7 estimator test) **has** now been run; §5.5(4) (cache-warmth sensitivity) has
  not.
- **The §3.7 estimator test is run** (`go-cascade estimator`, experiment 19). Mutation
  score M is a **conservative** proxy for 1 − η_fa on this benchmark, not a tight one:
  measured η_fa 0/144 (95% bound 0.021) against a pooled 1 − M of 0.0996 that predicted
  ~11 events. Non-circular by construction — M is measured against the *generated*
  suite, correctness against each problem's *human-authored* `refs/<id>/solution_test.go`,
  which `Router.SetCanonicalTests` supplies **off the acceptance path** (it must never
  become a ladder stage or an oracle; that would break invariants #4/#6). Two live
  cautions: whether M *ranks* candidates by η_fa is **unresolved** (no events in either
  M bucket), and the generated oracle's observed errors are **all over-rejections** —
  a sound-but-stricter-than-canonical suite costs escalations, not risk, so it is
  invisible to the certificate.
- The judge-oracle comparison arm (§5.5c) is implemented **and now run live** (not
  only mock). `calibrate --oracle=judge` and `--compare` swap the acceptance oracle
  for an LLM reviewer (never a ladder stage — invariant #4) and record execution
  ground truth alongside the judge verdict, so `realized_risk` can be checked against
  `empirical_risk`. Live, the executable oracle certified strictly lower than the
  judge on identical candidates (α 0.19 vs 0.30 at n=64, δ=0.10), the gap driven
  mostly by the judge's over-*rejection*; its one confirmed over-*acceptance* was a
  scar-free race. (The old mock figure — judge certifies α=0.15 at 0 empirical while
  realized risk is 0.22 — still illustrates the §3.1 floor, but **is a mock number,
  not a measurement of any model.**)
- Adaptive conformal inference (paper eq. 9) is not implemented; only the static
  LTT bound plus shadow sampling.
- No boundary randomization, so the selected policy is feasible but not exactly
  optimal (§2.4).
- Non-nested tier ordering (Pandora's box, §2.6) is not implemented; tiers are
  assumed cost-ordered.
- Single-file, stdlib-only generation only.

## Style

Idiomatic Go. Comments explain *why*, particularly where a choice encodes a
statistical requirement — those comments are load-bearing documentation, not
decoration. `gofmt` and `go vet` must be clean before commit.
