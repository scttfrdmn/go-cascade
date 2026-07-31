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

## Result 3 — pinning the API: the floor was ~93% test noise

The `-refs` gate above could only adjudicate ~40% of the benchmark: references
use canonical names (`Fibonacci`, `GCD`) while the spec model invents its own
each run, so most references could not compile against the generated tests
(inconclusive). To close that, `calibrate -pin-api` extracts each reference's
exported signatures (`prompt.ExtractAPI`, via `go/ast`: bodies blanked to
`panic`, types/methods/generics/doc-comments preserved) and feeds them to the
spec model as a fixed contract, so it writes tests against exactly the names the
reference implements.

**Pinning collapsed the inconclusive rate from ~57% to 13%** (2:1:1, n=47 —
another externally-interrupted run, see limitations). Investigating the 13%
residual surfaced a *third* spec-model noise class: a generated test that uses a
stdlib package it forgot to import (`undefined: sync`/`strings`/`unicode` in a
`_test.go`). Such a test compiles against **nothing** — it refutes every
candidate *and* the reference (all six had cluster score 0.0 at every tier) — so
it is a broken oracle, not an API mismatch. The gate now disambiguates this
(`testsCompileAgainstOwnAPI`: if the tests do not compile against their own API
stub, unsound; if they do but the reference does not fit, inconclusive), reaching
a verdict on **100%** of the profiled problems.

The result after removing the noise the gate can now identify:

| stage | verdict breakdown | cascade empirical risk | certifies |
|-------|-------------------|------------------------|-----------|
| no gate | — | 0.15 | nothing useful |
| pinned + gate (n=47) | 7 unsound, 0 inconclusive, 40 clean | **0.025** | **α=0.15** |

Of the 7 excluded: **1** was a wrong-value assertion (`scale_fibonacci`:
`Fibonacci(79)` wrong) and **6** were missing-import broken tests. After
exclusion the cascade is wrong on exactly **one** sound-oracle problem
(`conc_safe_counter`, a genuine top-tier miss) — **empirical risk 0.025**.

**This reframes the study's most-cited earlier conclusion.** Experiment 8 said
"the deployable-α floor is model accuracy (~11%), not sample size." With the
oracle noise removed, the true model-error rate on this benchmark is **~1 in 40
(0.025)**, not 11%. The floor was **neither** model accuracy nor sample size — it
was **spec-model test noise**. Now that it is gone, **sample size is once again
the binding constraint**: α=0.05 needs n≥~45 clean records (paper eq. 7) and the
interrupted run left only 40.

## Honest limitations

- **The clean floor (0.025) is n=40, from an interrupted run.** Three of the four
  long live runs (`-refs` ×2, `-pin-api` ×1) were killed externally at ~45–50 of
  64 problems (`context canceled` on the tail); the two dirty runs and one 3:2:1
  `-refs` run completed. So the pinned floor rests on the 47 profiled problems (40
  clean after exclusion), not the full 64. n=40 is *below* the ~45 the paper needs
  for α=0.05, which is exactly why α=0.05 does not certify here — a sample-size
  wall, not an accuracy one. A completed n=64 pinned run is the remaining step, and
  it needs whatever is interrupting long jobs resolved first.
- **The gate's `-pin-api` reach depends on the spec model obeying the pin.** It
  collapsed inconclusive from ~57% to 13%, and the refined gate reclassifies the
  residual (missing-import tests) as unsound, reaching a verdict on 100% of
  *profiled* problems. But that is because pinning worked on this benchmark; a
  spec model that silently reworded a pinned signature would reappear as
  inconclusive. The mechanism is sound; the coverage is empirical.
- **The unsound set is unstable across runs** (only `scale_two_sum` recurs across
  `-refs` attempts) — the resampled-test-noise signature, visible inside the
  gate's own verdicts, and *why* cross-run risk numbers were never comparable.
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
2. **The measured risk floor was overwhelmingly spec-model test noise, not model
   accuracy.** A reference-validation gate — extended by pinning the API so it can
   reach a verdict on the whole benchmark — shows the true model-error rate is
   **~1 in 40 (0.025)**, versus a raw measured ~0.15 and experiment 8's cited
   ~0.11. Three distinct spec-model noise classes were found and are now detected:
   wrong expected values, mismatched function names, and missing imports.
3. **This relocates the deployable-α blocker back to sample size.** With the
   noise removed the cascade certifies α=0.15 at n=40 and would reach α=0.05 near
   n≥45 (paper eq. 7). Experiment 8's "the floor is model accuracy" was an
   artifact of noisy oracles.
4. **A cheaper bottom tier does not lower the floor** — the floor is a top-tier
   property under a sound oracle. The apparent "drop" is noise removal, not model
   improvement.

## Reproduce

```bash
# Cost baselines (offline, free) on the committed records:
go-cascade calibrate -from-records results/go-specialist-211.execution.json -alpha 0.19 -delta 0.10 -baselines -o /tmp/c.json
go-cascade calibrate -from-records results/go-specialist-321.execution.json -alpha 0.19 -delta 0.10 -baselines -o /tmp/c.json

# Certificate flip from the oracle-soundness gate (offline, on committed records):
go-cascade calibrate -from-records results/go-specialist-321-refs.execution.json -alpha 0.19 -delta 0.10 -o /tmp/c.json    # -refs: valid=true, 2 unsound excluded
go-cascade calibrate -from-records results/go-specialist-211-pinned.execution.json -alpha 0.15 -delta 0.10 -o /tmp/c.json  # pinned: valid=true, 7 unsound, 0 inconclusive, risk 0.025

# Live, with API pinning (needs Bedrock; ~$6/config). -pin-api implies -refs:
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.go-specialist-211.json \
  -bench examples/bench/combined.jsonl -refs examples/bench -pin-api \
  -alpha 0.10 -delta 0.10 -compare -baselines -records run.json
```

Live spend this experiment: ~$32 (two dirty runs; one complete + two interrupted
2:1:1 `-refs` runs; one complete 3:2:1 `-refs` run; one interrupted `-pin-api`
run). Session total: roughly $65–69. The committed `go-specialist-211-pinned`
records carry the shipped gate's flags (6 missing-import problems reclassified
from inconclusive to unsound per the refined `testsCompileAgainstOwnAPI` logic).
