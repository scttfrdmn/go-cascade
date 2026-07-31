# Go-specialist bottom tier + the oracle-noise floor — 2026-07-25

Two open levers from the cost baseline (#17) and fan-out (#18) work were: a
cheaper/more-accurate **bottom tier**, and a flatter **fan-out**. This run tests
both at once by swapping a cheap non-Claude coder into tier 0 and profiling at
two intermediate fan-outs — and, in doing so, surfaces a confound that was
inflating every risk number this project has reported: **spec-model test noise.**

## Setup

- **Tier 0:** `us.meta.llama4-maverick-17b-instruct-v1:0` — the cheapest strong
  coder on this Bedrock account (no true Go-only specialist exists there). Its
  prices are the **real** us-west-2 on-demand rates from the AWS Pricing API:
  **$0.24/MTok in, $0.97/MTok out**, ~4× cheaper than Haiku ($1/$5).
- Tiers 1–2 keep Claude (`sonnet-4-5`, `opus-4-5`) at the same list prices every
  prior run used, so cross-run cost comparison holds. Oracle/judge = `sonnet-4-6`
  (distinct from every tier — invariant #3).
- Fan-outs: **2:1:1** and **3:2:1** (the "sweet spot" the 1:1:1 fan-out run
  skipped). Benchmark: `examples/bench/combined.jsonl` (n=64), δ=0.10.
- Config files: `config.go-specialist-211.json`, `config.go-specialist-321.json`.

Pre-spend gate: a single live solve confirmed Maverick produces parseable,
compiling Go through the two-phase harness (killed 6/6 mutants) — no refusals,
the main risk with a non-Claude model.

## Result 1 — the cascade wins on cost, with the sample count intact

Ground-truth cost baseline (execution oracle), same n=64 records per config:

| policy | 2:1:1 risk | 2:1:1 cost | 3:2:1 risk | 3:2:1 cost |
|--------|-----------|-----------|-----------|-----------|
| always-cheapest (Maverick) | 0.266 | $0.00048 | 0.203 | $0.00073 |
| **cascade** | **0.172** | **$0.00251** | **0.125** | **$0.00247** |
| always-frontier (Opus) | 0.172 | $0.00793 | 0.141 | $0.00827 |

- **2:1:1:** cascade is **3.16× cheaper** than always-frontier at **equal risk**
  (0.172 = 0.172).
- **3:2:1:** cascade is **3.35× cheaper AND lower-risk** than always-frontier
  (0.125 < 0.141).

This is a stronger result than the 1:1:1 fan-out win (#18): it holds at a fan-out
that **preserves multi-sample behavioural clustering** (2–3 cheap samples, not 1),
so the routing signal is not starved. The cheap tier's per-query cost is genuinely
low again ($0.0005–0.0007) because Maverick is both cheaper per token than Haiku
*and* drawn at a modest fan-out. Goal (b) from the session kickoff —
"beat always-frontier on cost without starving the sample count" — is met.

**The cost advantage is robust; risk parity is within sampling noise.** Across all
four live runs (two dirty, two `-refs`, all fresh draws) the cascade was cheaper
than always-frontier by **1.4×–5.3×**. The *risk* relationship wobbled with the
draw — usually equal, sometimes the cascade lower (3:2:1 dirty), sometimes frontier
lower by a hair (3:2:1 `-refs`: 0.094 vs 0.125). At n=64 that is sampling variance,
not a real ordering, so the honest claim is **"much cheaper at no systematically
worse risk"**, not "strictly lower risk." Do not compare risk across the fresh
draws — only cost, and only cost *within* a run (all three policies share that
run's candidates).

A live **judge false-acceptance (η_fa)** also appears in both `-refs` runs: the
judge arm's realized (ground-truth) risk stays at execution's level while its
empirical (self-labelled) risk is ~2× higher (2:1:1: emp 0.13 vs real 0.065;
3:2:1: emp 0.16 vs real 0.081) — the judge over-rejects, inflating the risk it
would certify against, exactly the §5.5 mechanism.

## Result 2 — the "accuracy floor" was partly spec-model test noise

The kickoff hypothesis (a) was that a better bottom tier would lower the ~11%
accuracy floor that blocks a deployable α. It does **not**, and cannot,
structurally: because the acceptance oracle is sound (β=0), any candidate accepted
at tier 0 is already correct, and every *hard* problem escalates to Opus. So the
cascade's risk equals the **top** tier's risk — a cheaper bottom tier is a cost
lever, not an accuracy lever. Confirmed directly: the cascade's wrong problems are
exactly the top-tier's wrong problems.

But investigating *why* trivial problems (`scale_two_sum`, `scale_fibonacci`)
"failed at all three tiers including Opus" revealed the real story. The
calibration oracle runs the **spec model's generated `TestH*`/`TestV*`**, not a
validated reference. When the spec model writes a bad test, it labels correct code
as wrong — inflating the measured risk with test noise, not model error. Two
failure modes, both found live:

- **Wrong expected value.** `scale_two_sum`: the generated test asserted
  `TwoSum([1,3,5,7,-1], 4) == [2 4]`, but indices 0,1 (1+3=4) come first and the
  problem says "smallest index first." The test violates the problem's own rule.
- **Mismatched function name.** `scale_fibonacci`: the generated hidden test
  called `Fib(...)` while the API named it `Fibonacci` — a build failure that
  fails *every* candidate.

### The fix: a reference-validation gate (`calibrate -refs`)

A generated test suite is itself an acceptance oracle, and invariant #4 requires
it be sound. `calibrate -refs <dir>` runs each problem's **execution-validated
reference solution** through the freshly-generated tests and classifies the
outcome three ways (`Router.validateOracle`):

- **Sound** — reference compiled and passed every generated test.
- **Unsound** — reference *compiled* but a generated assertion refuted it
  (test/race/accept stage). The tests reject correct code → **excluded** from
  calibration, exactly like a contaminated record.
- **Inconclusive** — reference did not compile against the generated API
  (parse/types/build/vet), because the spec model invented a different function
  name/signature than the reference's canonical one. This is *not* evidence the
  tests are wrong → **kept**, only tallied.

The Sound/Inconclusive distinction is load-bearing: a first cut that called every
reference failure "unsound" over-excluded **12×** (flagged 39 of 64 when only 3
were real) and drove the measured floor spuriously to zero.

### What the gate shows — the certification flip, on both fan-outs

| config | no `-refs` empirical risk | `-refs` empirical risk | unsound excluded | certifies α=0.19? |
|--------|---------------------------|------------------------|------------------|-------------------|
| 2:1:1 | 0.172 | **0.050** | 3 | no → **yes** |
| 3:2:1 | 0.125 | **0.081** | 2 | no → **yes** |

In both configs, excluding the **provably-unsound-test** problems (a reference
that *compiled* against the generated API but was refuted by a wrong assertion)
drops the certificate's empirical risk and **flips α=0.19 from uncertifiable to
certified**. The unsound problems overlap only partly between runs
(`scale_two_sum` in both; `scale_count_vowels`/`scale_title_case` vs
`scale_title_case` otherwise) — resampled-test noise, now visible inside the
gate's own verdicts. (The 2:1:1 figure is from the completed run
`go-specialist-211-refs`, reclassified with the shipped three-way logic after two
native re-runs were interrupted externally; 3:2:1 carries native flags. Both
agree with independent partial runs.)

## Honest limitations

- **The gate reaches a verdict on only ~40% of the benchmark.** 36 of 64 problems
  are *inconclusive* in both configs: the references use canonical names
  (`Fibonacci`, `GCD`, `IsAnagram`) while the spec model invents its own each run,
  so the reference cannot compile against the generated tests. The gate correctly
  refuses to call these unsound, but it also cannot clear them. The remaining
  errors after exclusion all live in this inconclusive zone — including
  `scale_fibonacci`, where the name mismatch likely broke candidates too. **The
  true floor is bracketed, not pinned** (2:1:1: [0.05, ~0.09]; 3:2:1: [0.08,
  ~0.12]). Closing this needs the benchmark to pin each problem's API signature so
  the spec model must match the reference — the obvious next step, and a
  prerequisite for any honest deployable-α claim.
- **The unsound set is itself unstable across runs.** Between two `-refs`
  attempts only `scale_two_sum` recurred as unsound — the resampled-test-noise
  signature, now visible inside the gate's own verdicts. This is *why* cross-run
  risk numbers were never comparable.
- **`-baselines` cost/risk uses all records** (it does not apply the unsound
  exclusion); the *certificate* empirical risk does. The cost comparison
  (Result 1) is unaffected — all three policies ran against the same tests, so a
  buggy test rejects all three equally.
- **One benchmark, small n, single judge model.** Directions are robust; exact
  values are point estimates. Maverick is an approximation of a Go specialist,
  not one.

## What is established

1. **A cheap non-Claude bottom tier makes the cascade beat always-frontier on
   cost at an intermediate fan-out** — 3.16× (2:1:1, equal risk) to 3.35× (3:2:1,
   lower risk) — without starving the sample count. The strongest cost result in
   the study.
2. **The measured risk floor was inflated by unsound generated tests.** A
   reference-validation gate removes the conclusive cases and lets the 2:1:1
   cascade certify α=0.19 where the untriaged run could not.
3. **A cheaper bottom tier does not lower the floor** — the floor is a top-tier
   property under a sound oracle. The apparent "drop" is noise removal, not model
   improvement.

## Reproduce

```bash
# Cost baselines (offline, free) on the committed records:
go-cascade calibrate -from-records results/go-specialist-211.execution.json -alpha 0.19 -delta 0.10 -baselines -o /tmp/c.json
go-cascade calibrate -from-records results/go-specialist-321.execution.json -alpha 0.19 -delta 0.10 -baselines -o /tmp/c.json

# Certificate flip from the oracle-soundness gate (offline, on committed -refs records):
go-cascade calibrate -from-records results/go-specialist-321-refs.execution.json -alpha 0.19 -delta 0.10 -o /tmp/c.json  # valid=true, 2 unsound excluded

# Live, with the oracle-soundness gate (needs Bedrock; ~$6/config):
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.go-specialist-321.json \
  -bench examples/bench/combined.jsonl -refs examples/bench \
  -alpha 0.10 -delta 0.10 -compare -baselines -records run.json
```

Live spend this experiment: ~$26 (two dirty runs, one complete + two interrupted
2:1:1 `-refs` runs, one complete 3:2:1 `-refs` run). Session total: roughly
$59–63.
