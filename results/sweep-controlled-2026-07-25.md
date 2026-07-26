# Controlled judge-strictness sweep — 2026-07-25

The confounded sweep (`judge-sweep-2026-07-25.md`) re-sampled candidates per
strictness level, so it could not attribute η_fa/β movement to the knob. This
run fixes that: `ProfileStrictnessReplay` samples each problem once, runs
execution once, and judges the **same** representative at strict / balanced /
permissive. Only the tie-break instruction changes, so any verdict that flips is
the strictness knob alone. 6 defect-prone problems, α=0.10, zero skips. Records
in `results/sweep2.{execution,strict,balanced,permissive}.json`.

## Result

Execution baseline (all levels, same candidates): risk **0.0000** over n=6.

| strictness | judge-emp | judge-real | η_fa | β |
|------------|-----------|------------|------|---|
| strict     | 0.0000    | 0.0000     | 0    | 3 |
| balanced   | 0.0000    | 0.0000     | 0    | 3 |
| permissive | 0.0000    | 0.0000     | 0    | 2 |

Verdict flips across strictness (the isolated signal):

- **`slice_most_frequent` / mid tier: FAIL (strict, balanced) → PASS (permissive)**,
  on a *correct* program. Loosening the judge fixed one false rejection. This is
  the knob working as intended: β 3 → 3 → 2.
- All 17 other tier-instances were strictness-invariant — the judge was
  (correctly) confident, so the tie-break never engaged.

## What this establishes

- **The controlled design works.** With the candidate stream fixed, exactly one
  verdict flipped as the judge loosened, and it flipped in the expected
  direction (a false rejection recovered). Strictness is now cleanly isolated;
  the earlier sweep's confound is resolved.
- **β responds to strictness; a permissive judge rejects less.** Consistent with
  the knob's intent.

## What this does NOT establish — and why (honest)

- **The dangerous mode (η_fa rising with permissiveness) was untestable this
  run**, because execution found **every candidate correct** (baseline risk 0).
  With no wrong candidates in the stream, η_fa is forced to 0 regardless of
  strictness — that is benchmark luck, not evidence that a permissive judge is
  safe. The run simply did not present the judge with a wrong-but-plausible
  program to wave through.
- So the question "can loosening the judge raise η_fa?" remains **open**. The
  earlier confounded sweep *did* see η_fa=3 (on a wrong `seq_longest_run`
  candidate) but couldn't attribute it; this controlled sweep *can* attribute
  but had no wrong candidate to attribute. We need both at once.

## The clean follow-up

Run the controlled replay on a stream that is *guaranteed* to contain wrong
candidates — e.g. seed the sample with a known-defective solution per problem
(inject the `>=`-for-`>` variant), or select problems/temperatures where the
models reliably produce a subtly wrong candidate, then judge that fixed wrong
candidate at every strictness level. Only then can we see whether permissiveness
converts a strict-FAIL (correct rejection of wrong code) into a permissive-PASS
(η_fa) — the actual §3.1 danger.

## Combined read across all four live runs

| run | design | key result |
|-----|--------|------------|
| pilot (easy, #7) | independent-sample compare | confounded; exec sound |
| paired pilot (#8) | shared-stream compare | exec β=0 live; judge η_fa=3 (a race), β=8 |
| hard tier (#9) | shared-stream compare | judge over-rejected subtle code; η_fa=0, β=6 |
| sweep confounded (#10) | re-sample per level | η_fa=3 seen but not attributable |
| sweep controlled (this) | replay per level | knob isolated; β responds; η_fa untestable (no wrong candidates) |

Consistent thread: **execution is sound in every run (realized == empirical)**;
the judge is noisy in both directions; and the §3.1 *danger* is real in
principle (η_fa>0 was observed) but was never both isolated and non-trivial in
the same run. Closing that gap is the remaining experiment.

Live spend this session: roughly $18–22 across four experiments and diagnosis.
