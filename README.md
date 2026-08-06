# go-cascade

[![ci](https://github.com/scttfrdmn/go-cascade/actions/workflows/ci.yml/badge.svg)](https://github.com/scttfrdmn/go-cascade/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/scttfrdmn/go-cascade.svg)](https://pkg.go.dev/github.com/scttfrdmn/go-cascade)

Route a Go coding problem to the cheapest model that **provably** solves it.

> **Status: research prototype.** The statistical machinery is unit-tested
> against closed forms and the verifier ladder is measured, but the system has
> never been run against a live model — every behavioural number below comes
> from a deterministic mock whose defect distribution was stipulated by hand.
> See [§5 of the paper](docs/verification-saturated-cascades.md) for a full
> account of what is and is not established.

Candidates are not scored by a judge model. They are compiled and executed
against a test suite that was written, before any solution existed, by a
different model than the one writing the code. A failed verifier stage is a
*sound refutation* — it carries no probability — so adding verifier stages can
only reduce cost at fixed risk. That is the property the whole design turns on,
and it is not available to any confidence-based gate.

```
                 ┌── arm zero ──┐
problem ──▶ spec ──▶ cache ──▶ small ──▶ mid ──▶ large ──▶ accept
            (oracle)  │          │        │        │        (held-out
                      │          └─ repair┴─ repair┘         partition)
                      └─ verified transfer, not predicted
```

## Why an executable oracle changes the guarantee

With an LLM judge, the false-pass rate `η_miss` is unknown, and recovering true
risk from observed risk requires knowing it — which requires the ground truth
you were trying to avoid. The certificate is circular, and you cannot honestly
certify below the judge's noise floor.

With execution, `η_fa = 0` by soundness, and `η_miss` is the test-suite gap,
which is **directly estimable by mutation score**. `go-cascade` reports it on
every run.

## Measured stage costs

Warm build cache, single vCPU (Xeon @ 2.8GHz), Go 1.26.5. These are measured,
not assumed — the ladder ordering is derived from them:

| Stage | What it refutes | Cost |
|---|---|---|
| `V0:parse` | syntax | ~0.1 ms (in-process) |
| `V0:stdlib` | non-stdlib imports | free (AST) |
| `V1:types` | **hallucinated APIs, wrong signatures** | **~1 ms** (in-process) |
| `V2:build` | codegen | 43 ms |
| `V3:vet` | oracle/solution mismatch, `lostcancel`, `copylocks` | 113 ms |
| `V4:test` | functional defects (visible partition) | 120 ms |
| `V5:race` | happens-before violations | **1373 ms** (gated) |
| `V6:bench` | allocs/op ceiling | opt-in |
| `VA:accept` | held-out partition | 120 ms |

Three findings from building this that contradict the obvious design:

1. **`go build` does not compile `_test.go` files.** A signature mismatch
   between the solution and the oracle survives it and dies at `go vet`. That is
   what earns vet its 113 ms: it is the cheapest stage that typechecks the tests
   *against* the solution.
2. **`vet` costs more than `build`,** so build goes first — the opposite of the
   conventional ordering. Building first fails codegen errors at 43 ms and warms
   the cache for everything after.
3. **A shared, warmed `go/types` source importer is ~40× cheaper than spawning
   `go build`** (1 ms vs 43 ms) and still catches `undefined: slices.MaxRunFunc`.
   The importer caches stdlib packages across candidates, turning a 237 ms cold
   check into a 1 ms warm one.

The race stage is **32× a plain test run**, which is why it is gated on a free
AST predicate (`go`/`chan`/`select`/`sync`/`atomic`). The gate is deterministic,
so skipping costs nothing against the risk budget.

Economics: at ~800 output tokens a frontier call is ~$10⁻², while a 1s build is
~$2×10⁻⁵ — so `c_verify / c_model ≈ 0.004`. A verifier that changes the
escalation decision more than ~0.4% of the time pays for itself. **This is a Go
argument, not a generic one.** In Rust or C++ the same stage runs 10–60s and
lands near a mid-tier model call, forcing a different topology.

## Documents

| | |
|---|---|
| [`docs/verification-saturated-cascades.md`](docs/verification-saturated-cascades.md) | The paper: derivations, proofs, and the evaluation-limits section |
| [`CLAUDE.md`](CLAUDE.md) | Project instructions and the invariants that must not be broken |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Ground rules and ranked high-value contributions |

## Install

Requires Go 1.26+. A `go` toolchain must be on PATH **at runtime** — the
verifier shells out to it. AWS credentials with Bedrock access are needed only
for `--provider=bedrock`.

```bash
go install github.com/scttfrdmn/go-cascade/cmd/go-cascade@latest
# or
make build          # -> ./bin/go-cascade
make demo           # end-to-end run, no credentials required
```

## Use

```bash
# Model IDs churn. Discover what your account actually has:
go-cascade models --region us-west-2 --filter anthropic

# Solve, targeting 5% risk
go-cascade solve --alpha 0.05 "Implement a bounded worker pool that maps f over a slice, preserving order."

# Latency-bounded: flips from sequential cascade to speculative parallel
go-cascade solve --deadline 20s "..."

# Add deterministic objectives (measurements, so they cost nothing against risk)
go-cascade solve --max-complexity 12 --max-allocs 4 "..."

# Full trace, including per-tier behavioural clusters
go-cascade solve --json "..." | jq .trace

# No credentials needed: exercises the whole pipeline against real compilation
go-cascade solve --provider=mock "Return the longest strictly increasing run in a slice of ints."
```

### Earning the guarantee

Out of the box, thresholds are **priors, not a bound**, and every run says so:

```
risk    UNCERTIFIED: thresholds are priors, not a calibrated bound; oracle gap ~0% (mutation score 1.00 over 8 valid mutants)
```

To get a real certificate, calibrate on a benchmark of problems:

```bash
go-cascade calibrate -bench problems.jsonl -alpha 0.05 -delta 0.10 -o thresholds.json
go-cascade solve --thresholds thresholds.json --alpha 0.05 "..."
```

`problems.jsonl` is one `{"id": "...", "problem": "..."}` per line.

Calibration runs **every tier on every problem** rather than running the
cascade, so any threshold vector can be replayed offline and the grid search
costs nothing beyond the one-time collection. Running the cascade instead would
only ever observe the tiers the current thresholds happened to reach.

The result is Learn-then-Test: a grid of threshold vectors, a Hoeffding–Bentkus
p-value per vector for `H₀: risk(τ) > α`, and multiplicity control
(fixed-sequence over a **data-independent** ordering, or Bonferroni). You get

```
P[ risk(τ̂) ≤ α ] ≥ 1 − δ
```

distribution-free and finite-sample, assuming only exchangeability. The
confidence score need not be calibrated — only monotone in correctness.

**If no threshold vector can certify, the tool says so and refuses**, rather
than emitting a threshold anyway.

Because every tier was recorded on every problem, you can re-evaluate any alpha
offline without querying a model again:

```bash
go-cascade calibrate -from-records records.json -alpha 0.15 -o cert.json
```

### Executable oracle vs. LLM judge

The central comparative claim — that an executable oracle certifies below a
judge's noise floor (§3.1) — has a runnable harness. `-compare` profiles the
benchmark twice, once accepting on the held-out tests and once on an LLM
reviewer that reads the code but cannot run it, and prints what each *certifies*
against what it actually *delivers*:

```bash
go-cascade calibrate --provider=mock -bench problems.jsonl -alpha 0.15 -compare
```

```
arm         valid  cert-α    emp-risk  real-risk verdict
execution   true   0.150     0.0000    0.0000    certified risk holds
judge       true   0.150     0.0000    0.2222    NOMINAL ONLY: realized risk exceeds α (judge noise floor)
```

Both arms report zero *empirical* risk and issue a valid certificate. The judge
arm's *realized* (ground-truth) risk is higher because it passes a defect that
only the hidden partition catches — that gap is η_fa, which a judge cannot see
and so cannot certify against. **These are mock numbers**: the mock's judge
misses exactly the class of defect the design says it should, which exercises
the mechanism but measures no real model. The judge oracle is never a verifier
ladder stage (that would break soundness); it replaces only the acceptance
decision. Use `--oracle=judge` to calibrate a single judge-arm certificate,
`--judge-model` to pick the reviewer.

### What calibration actually found

Running this on an 18-problem benchmark changed the design twice, which is the
argument for building the calibrator rather than assuming the thresholds.

**Raw cluster mass is not a usable score.** The first run refused to certify at
any alpha, with a floor of 27.8% empirical risk. The records showed why: the
mid tier reported a score of exactly 1.0 on all 18 problems while being wrong
39% of the time. With two samples, a defect invisible to the visible partition
clusters with the correct solution, so mass is 1.0 unconditionally and a
threshold has nothing to bite on.

The fix is to report a **Wilson lower confidence bound** on the cluster mass
instead of the raw fraction. Unanimity among two samples and among five are both
mass 1.0, but they are not the same evidence:

| verified / sampled | raw mass | reported score |
|---|---|---|
| 1 / 1 | 1.00 | 0.27 |
| 2 / 2 | 1.00 | 0.42 |
| 5 / 5 | 1.00 | 0.65 |
| 10 / 10 | 1.00 | 0.79 |

That makes the statistic monotone in evidence rather than in sample luck, so a
tier that has not earned confidence escalates. Empirical risk dropped from 0.278
to **0.000**.

**Sample size is the binding constraint, not model quality.** Even at zero
observed errors, n=18 gives p = 0.9¹⁸ = 0.1501, so alpha=0.10 at delta=0.10 is
unreachable. The certificate becomes valid at alpha=0.15 (p=0.054) and misses by
a hair at alpha=0.12 (p=0.1002). To certify alpha=0.10 with zero errors you need
**n >= 22**; for alpha=0.05, n >= 45. Budget calibration problems accordingly.

## The knob

There is one: **λ**, the slope of the cost–risk frontier in dollars per unit
error probability. Min-cost-subject-to-risk and min-risk-subject-to-budget are
Lagrangian duals, so `--alpha` and `--budget` are two descriptions of the same
constraint and setting both is rejected.

Cost is a vector, though, and the components disagree. Dollars say *verify
aggressively, escalate rarely*; latency says *stop at V1 and escalate at once*.
That is why `--deadline` changes the topology rather than just the thresholds:
under a latency bound the optimal shape is not a cascade at all, because
parallelism buys latency at the price of dollars.

Additional objectives stack, and what they cost depends on how they are
established, not on what they measure:

| | Example | Risk budget cost |
|---|---|---|
| Deterministic | cyclomatic complexity, allocs/op, imports | **zero** — it is a measurement |
| Stochastic | wall-clock benchmarks, race coverage | own conformal group |
| Judged | maintainability, API taste | inherits judge noise floor |

Only the first kind is implemented as a gate. Use ε-constraints, not weights:
scalarizing to `Σ wⱼfⱼ` only reaches the convex hull of the Pareto frontier, and
non-convex regions are unreachable at *every* weight vector.

## The cache is arm zero, not a front-end

Cache entries are keyed by canonicalized AST hash and retrieved by trigram
similarity, but **retrieval is never trusted**. A retrieved solution is
re-executed against the *new* query's tests. A hit is therefore a verified
transfer, not a predicted one, and contributes nothing to the risk budget. A key
collision costs one wasted verification, not a wrong answer.

Three layers:

- **solutions** — prior code, re-verified on every reuse
- **specs** — API contracts and test partitions, keyed exactly. Test generation
  is `O(1)` per query while candidate generation is `O(tiers)`, so this is the
  highest-leverage layer. Observed: a spec-cache hit cut a run from $0.0084 to
  $0.0028.
- **failures** — refuted canonical forms, fed forward as negative constraints

Admission is gated on `cache_admit_score`, which defaults to unanimity at the
cascade's **narrowest** fan-out (0.2698 for a one-sample tier). That number looks
alarmingly low and the reason is worth stating, because the obvious alternative
was shipped for twenty-one experiments and was silently broken.

The routing score is a Wilson **lower bound** on cluster mass, not the raw mass
(invariant #9). A unanimous tier of 5 therefore reports 0.6488 and a unanimous
tier of 1 reports 0.2698 — never 1.0. The old default of 0.90 sat above the
ceiling of *every* shipped fan-out, so the admission branch was unreachable and
the solutions layer never held a single entry. Nothing failed: an empty cache just
escalates, which is indistinguishable from a cold one. `Config.Validate` now
rejects an unreachable threshold outright, and a test asserts a solve admits.

Nor can the bar be keyed to the widest tier, which is the tempting fix. Acceptance
most often lands on the **final** tier — both the narrowest and, by construction,
the one with no threshold at all (invariant #6) — so a higher bar admits nothing
on exactly the fully-escalated queries where reuse would pay for itself.

Admitting on thin evidence is safe here in a way it would not be on the acceptance
path, because arm zero re-executes every hit against the new query's tests
(invariant #5). A weak admission costs one wasted verification, never a wrong
answer; retrieval quality is a cost question, not a risk one.

So the honest statement is *not* that admission is uniformly stricter than
acceptance — the final tier accepts unconditionally, and nothing is stricter than
that. It is that admission is gated where acceptance is thresholded, and that the
gate has a computable ceiling.

### The trap this avoids

A warm cache absorbs the head of the query distribution, so the router sees the
**conditional distribution of cache misses** — not the distribution you
calibrated on, and it drifts as the cache warms. Exchangeability breaks and the
guarantee is void.

`go-cascade` routes a random `shadow_rate` fraction (default 5%) *past* the cache
and calibrates on that unbiased stream. Certificates report whether they had
enough shadow records, and say so when they did not.

## Oracle integrity

- Tests are generated **before** any solution exists, by `test_model`.
- If the accepted tier and `test_model` are the same model, the run is flagged
  `oracle_contaminated` and **excluded from calibration**.
- The suite is split: `TestV*` (visible) drives repair; `TestH*` (hidden) is the
  acceptance oracle and is **never** shown to a repair prompt.
- When the representative fails acceptance, the router **escalates rather than
  trying another candidate**. Shopping through candidates until one passes the
  held-out tests is selection against the holdout and would inflate true risk
  relative to the certificate. This costs money and is deliberate.
- Mutation score is reported per run as the oracle-gap estimate.

## Repair vs. escalate

Three actions, not two: `{accept, repair, escalate}`. The rule is marginal —
take whichever buys more correctness per dollar:

```
max( Δp_repair / c_repair , Δp_escalate / c_next )  ≷  1/λ
```

Repair dominates early because `c_repair ≈ c_k ≪ c_{k+1}` and the compiler
diagnostic is precise. Depth is capped at 2 because repair attempts on a fixed
model are strongly positively correlated: if it cannot fix the defect in two
rounds it will not fix it in five. Diagnostics are carried forward so the
expensive tier starts informed rather than cold.

## Security

**This tool executes model-authored code.** `GOPROXY=off` and stdlib-only
generation bound the dependency surface, and every stage runs under a timeout,
but nothing here sandboxes the code at runtime. In any untrusted setting use:

```bash
go-cascade solve --exec-wrapper "firejail --net=none --private" "..."
```

or run the whole thing in a disposable container.

## Cost attribution

`--provider=bedrock` runs bill under a **tagged application inference profile**,
so a shared AWS account can separate this tool's spend from everything else in
it. On by default:

```bash
go-cascade solve ...                        # billed under Project=go-cascade
go-cascade calibrate ... -cost-tag exp-31   # a per-experiment value
go-cascade solve --cost-tag "" ...          # opt out; bare model IDs, untagged
```

On first use of a model the provider creates one profile per `(tag, model)`
pair, tags it, and passes its ARN as the Converse request's model. Creation is
idempotent — the profile name is derived from the tag and model ID, so later runs
reuse it rather than fragmenting the spend across rows. If it can't (say
`bedrock:CreateInferenceProfile` isn't granted), the run **proceeds untagged with
a warning** rather than failing: a lost sample is worse than a lost invoice line.

Three things worth knowing:

- **The tag key is `Project`, not something go-cascade-specific.** A cost-allocation
  key only breaks spend down after it is *activated*, and activation does not
  backfill — spend recorded before it stays unattributed forever. Reusing an
  already-active key means attribution starts with the first run.
- **`ConverseInput.RequestMetadata` is not the mechanism**, despite looking like
  it. It filters *invocation logs*, not bills, and would have attributed nothing
  while returning no error.
- **Untagged spend is unrecoverable after the fact.** That is why this defaults
  on rather than being opt-in: the failure mode is silent, one-way, and only
  discovered when you try to reconcile.

`go-cascade models` lists both system-defined and application profiles, so the
profiles this creates are visible there (it still does not list the bare-ID
open-weight catalog — see the Disclaimer).

The unit tests are fake-based and never touch AWS. One live check is opt-in,
because whether the runtime *accepts* a profile ARN is not something a fake can
establish:

```bash
GO_CASCADE_LIVE_SMOKE=1 AWS_REGION=us-west-2 AWS_PROFILE=... \
  go test ./internal/model/ -run Smoke -v
```

It spends ~$0.01 (two 24-token completions, one per ARN family) and creates two
profiles under a separate `smoke-test` tag value, so a probe never lands in an
experiment's cost row. **Run it twice** — the first call exercises profile
creation, the second exercises lookup, and they fail independently. It does not
verify the attribution itself: Cost Explorer lags about a day, so that is a
next-day query under `Project=smoke-test`.

## Where it breaks

- **Verifier hacking.** Optimizing against tests produces code that passes
  tests. Mitigated by the separate test author and the held-out partition;
  monitor mutation score as the health metric for the oracle itself.
- **Clustering is optimistic.** Candidates that agree on the visible partition
  and differ only on held-out behaviour land in one cluster, inflating mass.
  Observed directly: a `>=`-for-`>` defect clustered *with* two correct
  solutions. The Wilson bound limits how much confidence a small sample can
  claim, but it does not fix the underlying blindness -- if the visible
  partition cannot see a defect, no amount of sampling will. The diagnostic is
  the mutation score of the *visible* partition specifically; when it is low,
  treat cluster mass as uninformative regardless of how many samples agree.
- **Race detection is sound but incomplete.** ThreadSanitizer has no false
  positives but only sees executed interleavings. `-race -count=N` narrows it; a
  latent happens-before violation can still pass. Treat concurrency tasks as
  their own conformal group.
- **Marginal, not conditional.** You get average risk ≤ α over the query
  distribution. Generics- and reflection-heavy code can sit systematically wrong
  inside a passing average. Use Mondrian/group-conditional conformal with
  explicitly declared groups if that matters; you pay in calibration sample size
  per group.
- **Non-functional correctness is mostly invisible.** The cascade will accept an
  O(n²) solution. `--max-allocs` recovers part of it; API taste is not in the
  oracle at all.
- **Static drift.** Model IDs and prices are configuration, not truth. Verify
  with `go-cascade models` and your own pricing page.

## Layout

```
cmd/go-cascade        CLI: solve, calibrate, models, cache
internal/cascade     the router: arm zero, sampling, repair, escalation
internal/verify      verifier ladder, workspaces, mutation testing
internal/cluster     behavioural clustering by test-outcome vector
internal/cache       arm-zero cache: solutions, specs, failures
internal/calibrate   Learn-then-Test, Hoeffding–Bentkus, certificates
internal/model       Bedrock Converse provider + deterministic mock
internal/prompt      two-phase prompts and reply parsing
internal/config      tiers, cost model, risk knobs
```

## Prior art

The individual pieces are known: model cascades, Learn-then-Test
(Angelopoulos et al.), conformal risk control, Pandora's box for non-nested
ordering, semantic caching, mutation testing. What is composed here is
*sound refutation as a zero-risk cascade branch* — verification that can only
reduce cost at fixed risk — together with cache-as-arm-zero under a single joint
risk budget and the miss-distribution correction. Worth a literature check
before claiming novelty for that combination.

## Disclaimer

This is a personal research project. It is not affiliated with, endorsed by, or
a product of any employer. The per-token prices in `internal/config` are
**illustrative** and go stale; they are not a quote. Model IDs are guesses that
drift — discover the real ones for your account with `go-cascade models`. Nothing
here has been run against a live model (see the status banner above), so every
behavioural number comes from the deterministic mock. Use at your own risk under
the Apache-2.0 license; running `--provider=bedrock` incurs real AWS charges that
are entirely your responsibility.

## License

Apache License 2.0. Copyright 2026 Scott Friedman. See [`LICENSE`](LICENSE).
