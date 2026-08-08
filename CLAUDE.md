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
   affects only how often the cache is worth consulting. *Corollary:* because every
   hit is re-executed, admitting on thin evidence is a **cost** question, not a risk
   one — which is why `cache_admit_score` defaults to unanimity at the *narrowest*
   fan-out (`config.DefaultAdmitScore`) and not to something impressive-looking.
   `res.Score` is a Wilson lower bound (invariant #9), so it never reaches 1.0: the
   old default of 0.90 was above the ceiling of every shipped fan-out, `PutSolution`
   was unreachable, and the solutions layer was dead for 21 experiments with nothing
   failing — an empty cache escalates, indistinguishable from a cold one.
   `Config.Validate` now rejects an unreachable threshold. Do not raise
   `cache_admit_score` past what a tier can actually score, and do not key it to the
   *widest* fan-out: acceptance usually lands on the final tier, which is the
   narrowest and has no threshold at all (invariant #6).

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

- **A threshold compared against a bounded statistic needs its ceiling checked.**
  Swapping raw cluster mass for a Wilson lower bound (§4.3) made the score
  never reach 1.0, and every constant compared against it kept its old meaning. The
  cache-admission threshold silently became unsatisfiable. There is no test that
  fails for this: the gated branch just never runs. If you add a threshold on
  `res.Score`, compute the reachable ceiling (`cluster.UnanimousScore`) and assert
  the gated effect actually happens end-to-end, not merely that the code compiles.
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
- **A timeout is the one refutation whose cause can be external to the candidate**, and
  it is now marked as such — `verify.StageResult.TimedOut` → `Report.TimedOut()` →
  `cascade.Step.TimedOut` → `calibrate.TierObs.TimedOut`, with `CountTimedOut` and a
  `Certificate.NTimedOut` tally printed by `calibrate` (issue #63). The **scoring is
  unchanged and must stay unchanged**: `stageFrom` keeps `OK: res.Err == nil &&
  !res.TimedOut`, because a program that does not finish is not a program that works and
  a timeout-is-inconclusive rung would make a ladder stage probabilistic (invariant #4).
  The flag is **forensic only**, same constraint as `TierObs.DisagreementSource` — nothing
  on the acceptance path may branch on it, and no record is *excluded* for carrying it
  (that would select the calibration sample on an outcome, invariant #8). Sampling is
  covered as well as acceptance, and deliberately: a timeout during sampling refutes that
  candidate, which shrinks its behavioural cluster and so moves the very score a
  calibrated threshold is compared against, so load can change a routing decision by a
  path that acceptance-only recording would miss. Two asymmetries to preserve: the count
  is over **records, not observations** (three timed-out tiers on one problem is one
  suspect problem, not three events), and a live run prints its tally **even at zero**
  while `printCert` stays silent there — `TimedOut` is `omitempty`, so in a replayed file
  "0" and "never recorded" are the same bytes. This matters most for the concurrency set
  (#50), where every problem runs the `-race` rung on the study's smallest sample.
- **Bedrock model IDs churn.** They are configuration, never constants. Use
  `go-cascade models` (calls `ListInferenceProfiles`) to discover real ones — but note
  it lists **only `us.*` inference profiles**. The open-weight catalog (Qwen, DeepSeek,
  Kimi, GLM, Devstral — 112 models as of 2026-08) uses *bare* IDs like
  `qwen.qwen3-coder-30b-a3b-v1:0`, so `models` reports them as absent when they are
  not. `aws bedrock list-foundation-models --region us-west-2` is the ground truth. All
  of them answer through the same `bedrock-runtime converse` path
  `internal/model/bedrock.go` already uses; there is **no second endpoint** to
  integrate, so a new tier-0 model is a config file. Verify a rate against the Pricing
  API (`regionCode=us-west-2`) and a real per-sample cost against a live probe before
  configuring one — for reasoning models output *length*, not the per-token rate,
  drives cost, and a "cheap" reasoning tier 0 can outspend the Claude mid tier.
- **No `Record` stores the spec cost, so no cost figure derived from records is the
  bill.** Every `r.spec(...)` caller passes a throwaway `&Result{}`, and the shared
  oracle is **91% of real spend** (~$197 actual vs ~$40 across `results/*.md`; measured
  $0.0408/problem). Paired ratios are unaffected — the term cancels — which is why this
  survived 23 experiments. Absolute figures are not. Before quoting spend, reconcile
  against a past *bill*, and query **both** Cost Explorer service lines: Claude bills
  under `Amazon Bedrock Service`, `Amazon Bedrock` holds only the open-weight models
  (filtering on the latter alone totals $1.14 for the whole study). That service line
  also contains Claude Code's own usage — filter by `USAGE_TYPE`. See `results/README.md`
  §"A correction to every cost figure on this page".
- **A model ID becomes an ARN at the provider boundary and nowhere else** (issue #74,
  `internal/model/costtag.go`). Bedrock spend is attributed by billing against a
  **tagged application inference profile**, whose ARN is what `ConverseInput.ModelId`
  carries; `-cost-tag` (default `go-cascade`, `""` disables) sets the value of the
  `Project` cost-allocation tag. Why this is defaulted **on**: untagged spend cannot be
  attributed after the fact, so a forgotten flag costs a reconciliation permanently —
  this account's `Amazon Bedrock Service` line ran $1126.23 in one day, entirely
  untagged, against a whole-study total of ~$197, and experiment 30's ~$1.20 is
  unrecoverable for exactly this reason. Four traps, three of which produce no error:
  **(a) `ConverseInput.RequestMetadata` is not it.** It is a `map[string]string` on the
  struct this package already builds, so it looks like the answer; its own doc says it
  filters *invocation logs* — and logging is not even configured here. It would have
  attributed nothing, silently. **(b) Substituting the ARN upstream of the provider
  disables invariant #3.** `Router.contaminated` enforces it by **string equality** on
  model IDs (`tier.ModelID == cfg.TestModel`), and in the default config that comparison
  fires today (tier `mid` and `TestModel` are both claude-sonnet-4-5). One model spelled
  two ways means `OracleContaminated` never sets and contaminated records enter
  calibration, with **no test failing** — the guarded branch just stops running. Hence
  `config.Tier.ModelID` keeps the logical ID everywhere and resolution happens one
  function above the wire, pinned by
  `TestContaminationComparesLogicalModelIDs`. **(c) A cost-allocation key must be
  ACTIVE and activation does not backfill**, so the key is the account's existing
  `Project` rather than a fresh go-cascade one — a new key would leave everything before
  activation unattributed forever. **(d) `ListInferenceProfiles` with no `TypeEquals`
  returns only the SYSTEM_DEFINED profiles**, so the obvious single call cannot see the
  account's own application profiles: an unfiltered lookup would create a duplicate
  profile every run and split one model's spend across rows. Same shape as the standing
  caveat that `models` omits the bare-ID open-weight catalog — an absence there is not
  evidence of absence. Two properties to preserve: resolution **never fails** (on any
  error it falls back to the bare ID, warns **once per model**, and caches the fallback —
  a lost sample is worse than a lost invoice line, and `CreateInferenceProfile` is a
  distinct IAM permission from `InvokeModel`), and the source ARN is **looked up, never
  assembled** — the two model families have different ARN shapes (`us.*` Claude is an
  account-scoped inference-profile ARN, `qwen.*` an account-less foundation-model one),
  so a hand-built ARN works for one family and fails for the other.
- **A profile's `models` list is not the ARN you copied from, and only the SECOND run
  looks at it.** `CreateInferenceProfile` from a **cross-region** source stores the
  *underlying* foundation models, one per routable region, with the routing prefix
  **stripped**: copying `us.anthropic.claude-haiku-4-5-20251001-v1:0` yields three
  `…::foundation-model/anthropic.claude-haiku-4-5-20251001-v1:0` entries — no `us.`. So
  comparing the source's ARN tail against the profile's tails never matches for any
  Claude tier, `findProfile` rejects a profile **we ourselves created**, and the run
  falls back to untagged. It is invisible on the first run, which returns the ARN
  straight from creation and never consults `findProfile` — so the Claude tiers, **91%
  of real spend**, would be tagged exactly once and never again, with only a stderr
  warning. Hence `modelIdentity` folds the routing prefix, as an **allowlist**
  (`us.`/`eu.`/`apac.`/`global.`/`us-gov.` — this account uses two of them) and not by
  stripping the first dotted segment, since a *bare* ID's first segment is its vendor
  (`qwen.`, `anthropic.`) and folding that would equate two vendors' same-named models.
  Two process lessons, both cheap and both learned the hard way here: **a live probe
  must run TWICE** — one call exercises creation, the next exercises lookup, and they
  fail independently; and **a fake that is simpler than the API passes code the API
  rejects.** The original fake echoed the source ARN into `models`, which is why nine
  green tests and a merged PR did not catch this. It now reproduces the real shape
  (`modelsFor`), verified to fail against the old comparison before being trusted.
- **`AWS_REGION` in the environment overrides a test's own default**, so a probe can
  silently verify a region the study does not use — the first cost-tag smoke run
  checked `us-east-1` because the env said so, while `config.Default().Region` and every
  experiment are `us-west-2`. Both worked, but "it passed" was about the wrong region.
  Pass the region explicitly when a live check is meant to be evidence about a
  particular one.
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

- **§5.5(1) is met (experiment 21).** The paired comparison has been run at **n=409
  usable on a standard benchmark** (MultiPL-E Go, 488 problems): **execution certifies
  α=0.084, the judge α=0.226** at δ=0.10 on identical candidates, so the margin *widens*
  with scale (1.19× at n=28 → 1.58× at n=64 → **2.69×**). Execution sound on
  **1096/1096** observations. **η_fa measured for the first time: 11/1096**, gradient
  8/2/1 across cheap/mid/frontier — but *those* records keep no candidate source, so
  their defect classes are unrecoverable and the *reading-invisible* mechanism is still
  argued **for that run**. Fixed going forward: `TierObs.DisagreementSource`
  (`RetainSourceOnDisagreement`) keeps the program wherever an arm's oracle differs from
  execution truth, and `results/classify_disagreements.py` reads it back out
  (`--dump <dir>` writes one annotated `.go` per event). Retention is **forensic only** —
  nothing on the acceptance path may read it, or it becomes an unsound oracle input
  (invariants #4, #6) — and it is limited to disagreements, which at n=409 is 166
  programs rather than 1096. **Two n=64 headlines do not survive:** α=0.05 does **not** certify at n=409
  (floor 0.0538 is real model accuracy, not oracle noise — the n=64 "0/52 errors" was
  too small a sample to contain the model's errors), and the certifiable-α-vs-cost-win
  tension **reproduces** (`[1,1]` below α=0.11, `[0.1,1]` and 2.2× cheaper at or above).
  Prefer the n=409 figure wherever it conflicts with an n≤64 one. **New coverage gap:**
  MultiPL-E Go has **0 concurrency problems**, so the `-race` rung was never exercised
  at scale — the 64-problem hand-written set is not obsolete. Still open:
  single-file/stdlib-only (§5.4). **Open work is tracked on
  https://github.com/users/scttfrdmn/projects/62 (issues #49–#53), not in prose here.**
- **Earlier live evaluation** (20 experiments against Bedrock; see `results/` and paper
  §5.6). Those are **demonstrated, not validated**: n ≤ 64, below the §5.5 bar of
  n ≥ 300, on a small non-standard single-file/stdlib benchmark. Of the two secondary
  tests, §5.5(5) (the
  §3.7 estimator test) **has** now been run; §5.5(4) (cache-warmth sensitivity) is run
  under an *injected* shift (experiment 22, see below) but **cannot be run on an observed
  one on a benchmark of this construction**. Measured offline for
  $0 (`results/absorption_ceiling.py`, experiment 20): retrieval candidacy 464/488
  (95.1%) but the **absorption ceiling is 2/488 (0.4%)**, because arm zero re-executes
  (invariant #5) and lexical similarity does not imply transferability. A 0.4% cache
  shifts no distribution, so the paid run would have reported a corpus artifact as a
  null. Do **not** read this as licence to relax invariant #8 — it is a fact about
  independently-sampled benchmarks, not about warm caches; testing §2.9 needs duplicate
  injection (absorption as a controlled dial) — which is experiment 22. By-product, and the stronger result:
  similarity is *anti*-correlated with transferability at the high end — the top pairs
  are antonyms (`minimum`~`maximum` 0.949, the highest of 118,828), and the top
  same-signature pair (`he_56`~`he_61` `CorrectBracketing`, 0.836) is retrieved and
  **refuted** (`<>` vs `()`). Invariant #5 measured, not asserted.
- **§5.5(4) is run as a *dial*, not an observation** (`go-cascade absorption`,
  `internal/calibrate/absorb.go`, experiment 22). Absorption is injected into the
  recorded n=409 stream, which makes the whole sweep free — every tier is profiled on
  every problem, so any pattern × any threshold vector replays offline. Result: under a
  head-shaped filter the certificate goes optimistic monotonically (+0.0147 → +0.1486 as
  ρ goes 0.2 → 0.8) and at ρ=0.6 promises α=0.10 while delivering 0.134 — a *violated*
  bound, not a loose one. Three traps, each of which produced a wrong answer first:
  **(a) Uniform absorption is a null.** Dropping a random subset of an exchangeable
  sample leaves an exchangeable sample, so tier-0 accuracy does not move (0.7702 →
  0.7439 across all rates, noise in both directions). #52's own framing — inject exact
  duplicates uniformly at random — would have measured nothing and reported it as
  evidence about §2.9. `AbsorbUniform` therefore ships as an explicit **null control**,
  and its envelope is the yardstick the selective rows are read against. **Measure that
  envelope over seeds, not from one sweep**: easy-first is a sort and so seed-exact, but
  the uniform draw is random, and 10 seeds × 4 rates gives max |gap| **0.0389** where a
  single sweep showed 0.0267 — enough to move the ρ=0.4 verdict. By the honest envelope
  the effect holds at **ρ ≥ 0.6**, not ρ ≥ 0.4. Do not delete the uniform arm as
  redundant: the control's spread, not the treatment's magnitude, sets the bar. That
  estimate is `EstimateNullEnvelope` (`-null-seeds`, default 10) and it ships *inside*
  the output — a bar left to the reader gets compared against one draw of the control.
  Exclude underpowered and uncertified rows from it, or trap (c) inflates the yardstick.
  **(b) `Calibrate` prefers the shadow subset of whatever it is handed**, and every
  profiled record has `Shadow: true` (invariant #8). Left set, that preference fires on
  *both* arms and makes the uncorrected arm silently identical to the corrected one —
  reading as "shadow sampling has no effect." Hence `stripShadowFlags`: the sweep models
  the correction explicitly, by choosing which stream to calibrate on.
  **(c) Shadow sampling spends sample size**, so a small-ε row can look worse for
  reasons unrelated to any shift. `MinCalibrationSize(α, δ)` is the floor — at r̂=0 the
  Hoeffding term is exactly (1−α)^n, so certifying needs (1−α)^n ≤ δ, i.e. n ≥ 22 at
  α=δ=0.10. Rows under it are flagged `Underpowered`, never dropped: a missing row reads
  as "not swept" and an unflagged one reads as a finding. The clean comparison holds n
  *fixed* (ε=1, ρ=0.2, n_cal=327 for all three patterns): uniform certifies `[1,1]`,
  both selective patterns refuse. Note what the correction does and does not buy — it
  drives the gap to exactly zero and converts a silent violation into a visible
  **refusal to certify**; it does not rescue the cost win under heavy absorption. None
  of this licenses relaxing invariant #8; it is the evidence *for* it.
- **Arm (e) is implemented and RUN — §5.5(2) is closed** (`go-cascade selfconsistency`,
  `internal/calibrate/selfconsistency.go`, `internal/cascade/selfconsistency.go`,
  `internal/cluster/text.go`, experiments 23 (free) and 24 (paid, $18.77)). Arms (a)/(b)/(d)
  replay from the records because every tier ran on every problem; arm (e) cannot, because
  "matched cost" is a budget equal to the cascade's *realized* spend and the vote is over
  candidates the cascade never drew. What *was* free is whether the arm is well-posed, and it
  mostly is not: at τ=[1,1] the $0.0101/query budget buys median **49 / 2 / 1** samples at the
  profiled 2:1:1 fan-out, i.e. below a 3-way vote on **0.0% / 79.2% / 99.5%** of problems.
  **A frontier arm (e) at matched cost is always-frontier relabelled** — run as §5.5(2)
  literally specifies it (tier unnamed) it would have reported a degenerate configuration as a
  null about self-consistency, exactly experiment 22's trap. Hence `SelfConsistencyBudget`
  ships as a gate that **refuses `-sample` on a ruled-out tier**, and the budget is matched
  **per problem**: averaging would fund every problem alike and hide that the expensive ones
  are the degenerate ones. **Live result at the one surviving tier: execution wins, 0.915 vs
  0.806 paired over 366 rows, McNemar 45-vs-5, p=4.2e-09**, with ~50 text votes against 2
  behavioural ones. The mechanism is the **abstentions**: where the cluster abstained the text
  vote was right **1/43** while its vote mass moved only 0.661→0.604 — confidently wrong
  exactly where execution reports nothing, an inversion rather than a gradient. Agreement is
  uninformative (293 vs 290 of 313 agreed rows); the whole margin is the 53 disagreements
  (2 vs 45). Load-bearing design choices, all of which make the foil *stronger*: the vote is
  over normalised source (`cluster.TextKey` drops comments/formatting/import order) but must
  **not** approximate semantics, or the text vote becomes behavioural clustering and the §3.5
  contrast dies; `TextVote` returns **raw mass**, not a Wilson bound — invariant #9 governs
  the *routing* score crossing a calibrated threshold, and arm (e) crosses none; and it never
  consults a verifier to pick its winner. Both selectors are scored on the **same candidates
  at the same cost** (isolating the selector), and a cluster abstention is reported
  separately, never as a wrong answer — nothing surviving is a sound refutation (invariant #4)
  and an escalation in a real cascade. Two subtleties with teeth: `sampleTierN(..., n, seed0)`
  exists because Bedrock exposes **no seed for Claude**, so `prompt.CodeUser`'s `(sample N)`
  nonce IS the diversity mechanism — a second batch restarting at 0 would redraw the first and
  inflate a majority with duplicates it did not earn; and `cascade.minVote` mirrors
  `calibrate.MinVote` by test, since calibrate must not import cascade.
  **A paired comparison must print ONE denominator.** The live pass first reported the text
  arm over all 409 voted rows against the cluster over the 366 it did not abstain on — both
  figures correct, read together as a 0.19 gap where the paired one is 0.11. No test failed,
  because no value was wrong; the *shape* of the report was. `printSelfConsistencySummary`
  now leads with the paired line, prints discordant counts, gives the abstention rows their
  own rate, and labels the all-rows figure incomparable;
  `TestSelfConsistencySummaryPairsOnOneDenominator` asserts that shape. Records:
  `results/arm-e-feasibility-n409.json`, `results/arm-e-n409.json`; write-up
  `results/arm-e-live-n409.md`.
- **The §3.7 estimator test is run** (`go-cascade estimator`, experiment 19). Mutation
  score M is a **conservative** proxy for 1 − η_fa on this benchmark, not a tight one:
  measured η_fa 0/145 (95% bound 0.020) against a pooled 1 − M of 0.1014 that predicted
  ~12 events. Non-circular by construction — M is measured against the *generated*
  suite, correctness against each problem's *human-authored* `refs/<id>/solution_test.go`,
  which `Router.SetCanonicalTests` supplies **off the acceptance path** (it must never
  become a ladder stage or an oracle; that would break invariants #4/#6). Two live
  cautions: whether M *ranks* candidates by η_fa is **unresolved** (no events in either
  M bucket), and the generated oracle's observed errors are **all over-rejections** —
  a sound-but-stricter-than-canonical suite costs escalations, not risk, so it is
  invisible to the certificate.
  **Oracle strength is a measured quantity, not an assumed one.** Experiment 19's first
  pass ran the canonical suite through the ladder's *visible* stage, so `-run ^TestV`
  applied and 222 of 370 canonical tests never executed — every stage still returned a
  *sound* verdict on what it ran, so no test failed and the resulting number looked
  publishable. The re-run reproduced the 0-event headline but cut confirmed false
  rejections 11 → 4 on a disjoint problem set and surfaced 3 real `TestH*` refutations
  (an off-by-one at `MaxUint64`, an int64 overflow) the weak oracle called correct.
  Hence: `Ladder.RunAllTests` (unfiltered, deliberately off the acceptance path),
  `EstimatorObs.CanonicalTests` recording the executed count per row, and a
  zero-tests-ran stage treated as a **refutation** in both `Run` and `Accept`. Do not
  reintroduce a `-run` filter on an audit oracle. Records:
  `results/estimator-n64-full-oracle.json`; the superseded 40% run is kept as
  `results/estimator-n64.json` for comparison. Corollary caution: the two draws agree
  on only 159/192 rows, so **rejection-side rates are not stable at n=64.**
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
- **The scar-free race sweep was DECLINED at 9 seeds against a pre-registered bar of 10, a
  fourth operator moved the measurement to 10 and it CLEARED that bar — the bar never moved
  — and then it RAN and BOTH ARMS ARE NULL: scar-free 0/9, sync-deletion 0/27, every
  strictness level, Fisher p = 1.0** (experiment 30, `results/scarfree-sweep-n9.md`, ~32 min,
  ~$1.20). The sweep's own output states the test — *"a rate ABOVE the `-seed-kind=race` rate
  on the same problems is the §3.1 mechanism demonstrated rather than argued"* — and **both
  rates are zero, so there is no gap to test.** §3.1's reading-invisible mechanism stays
  **ARGUED** on its one live event; this was the branch the pre-registered arithmetic gave the
  larger probability, so a null is the outcome, not a surprise. **Three things the run taught
  that outlive its verdict.** (i) **The realized denominator is not the instrument's reach.**
  `ProfileSeeded` mutates a tier-0 **model draw**, so the arm drew **9 from 5 problems** where
  the reference harvest reaches 10 from 7 — and the problem sets differ in *both* directions.
  A near-equal total over fewer problems is more **concentrated**, and several mutants of one
  base are not independent draws, so the effective n is *below* 9 and ≤0.283 is optimistic.
  Quote the realized denominator, never the bar's. (ii) **Re-run a control in-session rather
  than citing one across sessions.** The deletion arm came back **0/27 from 8 problems**, not
  the **0/20 from 9** on file from 2026-07-25 — same operator, same benchmark, same config,
  different draw. Citing the old figure beside today's would have crossed model versions
  inside a two-arm comparison: the same error shape as *one denominator per paired comparison*,
  and invisible, because both numbers are real results in this repo. The verdict is unchanged
  only by luck (the critical value is ≥3 against both), and that is checkable *only* because
  the control was re-measured. (iii) **`-seed-kind` wrote no records, only stdout** — per-
  problem seed counts and mutant sources were unrecoverable and the run was not resumable. On
  a null that cost little; on a positive event it would have destroyed the only interesting
  follow-up (*what* did the judge miss?), which is exactly the gap `TierObs.DisagreementSource`
  closes on the paired arm. **Fixed** (`cascade.SeededRecord`, PR #73): `-records`/`-resume`
  now work, `KilledMutant`'s `Source`/`Desc`/`DataRace`/`PlainRefuted` are persisted per
  mutant with the per-level verdicts, checkpointed after every problem — a persistence path,
  not new instrumentation, and forensic only (nothing on the acceptance path reads a record
  back). Four properties are load-bearing, each because the obvious version loses something:
  the printed table is **derived from the records** rather than accumulated beside them, so a
  printed η_fa and a persisted one cannot drift; **zero-seed rows are recorded with a
  reason**, because which problems yield nothing is a coverage fact about the *operator set*
  (three of the eleven were structurally unreachable) and "no verified candidate to mutate" is
  a sampling failure where "no mutant compiled and was refuted" is a claim about the
  operators — skipping the row collapses two different nulls into an absence; a **resume
  across defect classes is refused**, since appending scar-free rows to a sync-deletion file
  pools the two rates into one denominator; and a **partially-judged row is kept but re-run**,
  because skipping it leaves the file short of the seeds it says it harvested and the
  shortfall reads as a smaller denominator, not as missing data. The one surviving asymmetry is **bound tightness, not observed rate**:
  9 seeds bound the scar-free class at ≤0.283 where 27 bound the control at ≤0.105 — the class
  §3.1 is about is the one we can least afford to sample, a fact about **operator reach**, not
  the judge. **Do not read the zeros as η_fa = 0.** The pre-run coverage arithmetic follows,
  and is what made the spend decision legible:
  (experiments 28 and 29,
  `results/scarfree-coverage-n11.md`, `results/deferred-escape-n11.md`,
  `results/scarfree_coverage.py`). On the 11-problem concurrency set: **16 AST sites → 10
  seeds** (compile + a real ThreadSanitizer report), 7 of 11 problems, split 4 `defer-wait`
  / 4 downgrade / 1 escape / 1 escape-defer. Against the sync-deletion control **0/20** the
  critical value is **≥3 of 10**, a null bounds η_fa at **≤0.259**, P(≥1 event) is 0.893 at
  η_fa=0.2. **Clearing a registered bar makes a run fundable, not authorized** — and n=10
  buys an *existence proof*, not a rate, so read the arithmetic before quoting a price. (Both
  steps then happened in order: priced at ~$1.20 — cheaper than the decline's ~$3, because the
  seeded path skips the cascade tier loop entirely — authorized explicitly, and run.)
  The tenth seed came from implementing the **deferred form** of the escape operator, which
  had been skipping every `Lock(); defer Unlock()` — the dominant Go idiom, so the operator
  had exactly one site on a concurrency benchmark. It needs a control-flow **veto** the
  plain form does not: undeferring an `Unlock` only unlocks on the fall-through path, so a
  `return` in the covered region **deadlocks** instead of racing — wrong class twice over,
  since a timeout's cause can be external to the candidate and a `Lock` with a `return`
  before its `Unlock` is a *visible* imbalance (the deletion arm's territory). A `return`
  inside a `func` literal is **not** an exit. That was found by *reading both mutants*, not
  by the DATA-RACE filter, which would have dropped the hang and reported one seed either
  way. **A formatting artifact is a scar:** `go/printer` lays a block out from its
  statements' *recorded source lines*, not slice order, so any reordering operator emits a
  blank line at the mutation site and **`gofmt` does not remove it** (it preserves author
  blank lines). 3 of 17 sites carried the tell; now 0 of 16, via `clearPositions` — scoped
  to the statements that **moved**, because clearing the enclosing block re-lays-out code
  the mutation never touched (52 printed lines → 49, a collapsed closure and a dropped
  blank line: a three-line scar replacing a one-line one) versus 52 → 52 scoped. Confined
  to the scar-free reordering operators; `collectRaceSites`/`collectSites` untouched, so
  arm comparability holds. **Seed quality changed more than the count did:** experiment 28
  found *all* 9 seeds were also refuted without `-race` — so the sweep could only have
  measured whether a reader notices a deterministically-wrong program — and it is now **9
  of 10 on a 12-core box, 8 of 10 on a 2-core one**, the new seed being the first that
  genuinely requires the interleaving. **`PlainRefuted` IS MACHINE-DEPENDENT and must never
  be pinned to an exact count.** It is the outcome of *running* a racing program, so
  available parallelism decides it: measured 10 plain runs per seed,
  `hard_conc_rate_limiter:38` (downgrade) is refuted 10/10 at 12 cores and **1/10 at 2**,
  while the escape-defer seed is 0/10 on both (undeferring widens the window
  unconditionally, so the claim itself is stable). An exact per-operator expectation
  therefore pins the *runner*: it passed locally 10/10 and failed CI on three consecutive
  merges (#75, #76, #78) with `downgrade: 1 seeds refuted ONLY under -race, recorded 0`.
  The test now asserts a **floor** (`escape-defer >= 1`, the load-bearing claim) and
  **logs** rather than fails when another operator crosses — a crossing means more seeds
  genuinely need the rung, so failing on it is failing on good news and invites "fixing"
  a real measurement by editing the expectation. Quote the machine alongside any
  plain-refutation figure. `PlainRefuted` is still recorded and **never filtered**.
  **Route 1 is measured and closed:** `ProfileSeeded` mutates a tier-0 **model draw**,
  never a reference, and that population gives **8 raw / 6 unique / 5 from
  execution-correct bases** — 9 recorded rows are 7 unique programs (`conc_safe_counter`
  3× byte-identically) and `ProfileSeeded` never checks base correctness, so a mutant of an
  already-wrong program would count as a seed. Quote the range, not the flattering end.
  §3.1's mechanism stays **argued** — experiment 29 sharpened the instrument, it did not
  run the sweep.
  **Two retractions worth reading before trusting any figure here, because both errors
  pushed toward declining:** (a) the `RWMutex` downgrade was reported "**structurally
  dead** — no `sync.RWMutex` in `examples/bench/`". That was an operator bug: it rewrote
  the call sites and left `var mu sync.Mutex`, on the reasoning (in its own doc comment)
  that "the build filter enforces the RWMutex rather than a type check". Deferring a type
  constraint to the compiler turns an unreachable operator into an empty result, and an
  empty result reads as a finding. Co-mutating the declaration gives it **4 seeds**, so
  "one operator carries the set" is retracted too. (b) the control was **0/17**, which is
  the *logic* arm over 6 **different** problems; sync-deletion on these 11 is **0/20**.
  That swap alone moved the critical value from 2 to 3 of 6 — and 2/6 was the row labelled
  "cannot resolve." Both are real results in this repo, which is why it survived a
  read-through; this is the same error as *one denominator per paired comparison*.
  **Pin the number the decision rests on, not the cheap one:** the first pass pinned only
  AST sites, which stayed correct at 15 throughout the period the seed count was wrong, so
  the suite was green on a false conclusion. `TestScarFreeSeedCountOnTheConcurrencyBenchmark`
  now pins seeds per problem and per operator, and `TestConcurrencyRefIDsMatchTheBenchFile`
  pins the problem count — the old guard keyed on a hardcoded id list, so adding the 12th
  concurrency problem the write-up itself recommends left every assertion passing on the
  stale 11. **That pinning is what flipped the verdict**, not a re-read: the seed constant
  sat at 9, and landing the deferred-escape operator *failed the test*, which is the only
  reason anyone noticed the bar was now met. Pin the decision number and a later change
  cannot quietly invalidate a conclusion.
  One caution survives both experiments unchanged: the scar-free arm requires an actual
  **DATA RACE**, not merely a `-race` failure (`conc_safe_counter`'s mutant returns
  `got 0 want 400` with no race — a deterministic wrong answer in this operator's clothes,
  counted as a seed by the first pass). **The only route left for raising n is widening the
  corpus**, since the model-draw route is closed and the deferred form is implemented — and
  it revives only the *escape* operators. Its hazard: authoring problems mutable *by these
  operators* tunes the benchmark to the instrument, so any extension must be written as a
  plausible Go exercise first, checked for operator coverage second, and identified as post
  hoc in the write-up.
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
