# Diagnosis: the α=0.05-vs-cost tension is one flaky cheap-tier answer — 2026-07-30

Experiment 12 ([`pinned-n64-complete`](pinned-n64-complete-2026-07-30.md)) found
that at α=0.05 the executable oracle certifies (0 risk) **but the certified
thresholds are `[1,1]`** — the cascade escalates every problem and so costs *more*
than always-frontier. Deployable α and the 3.2× cost win looked mutually exclusive
at the 2:1:1 fan-out.

This note diagnoses *why*, offline and for free, from the committed records
(`results/analyze_tension.py`). The tension is **not** a broad property of the
cheap tier. It reduces to **a single problem**, and that localises the fan-out
experiment to a sharp, testable hypothesis before any money is spent.

## The mechanism

The routing score is a Wilson lower confidence bound on the largest verified
behavioural cluster (`internal/cluster.Score`), deliberately *not* the raw cluster
mass (invariant #9). At a fan-out of 2, an answer that is unanimous across both
samples (mass 2/2) scores **0.425**, not 1.0 — the bound's ceiling at n=2. The
ceiling rises slowly with fan-out and **never reaches 1.0**:

| tier0 fan-out | unanimous-answer score ceiling |
|--------------|-------------------------------|
| 1 | 0.270 |
| 2 | 0.425 |
| 3 | 0.526 |
| 5 | 0.649 |
| 10 | 0.787 |
| 20 | 0.881 |

So a threshold pinned at 1.0 is unclearable at *any* finite fan-out — the cheap
tier can never be accepted. The question is whether α=0.05 *forces* τ0=1.0.

## Why α=0.05 forces τ0 = 1.0 — one problem

On the 52 clean (gated) records, the tier-0 (Maverick) score distribution is:

```
tier0 score | truly correct: {0.425: 40, 0.121: 6}
tier0 score | truly wrong:   {0.425: 1,  0.121: 1}
```

Two facts matter:

1. **Zero tier-0 acceptance-risk events.** Not one answer the *oracle* passed at
   tier 0 (`correct=True`) was truly wrong. The cheap tier, when it says "done,"
   is right. So early acceptance carries no *hidden* danger here.
2. **One empirical-risk blocker.** `scale_chunk`: Maverick's answer was
   oracle-*refuted* (`correct=False`) yet its cluster still reached the unanimity
   ceiling **0.425** — both samples produced the *same wrong* output. It is
   score-indistinguishable from the 40 correct unanimous answers, which also sit
   at 0.425.

LTT's `Risk()` counts a tier-0 acceptance as bad whenever the oracle refuted it.
To keep `scale_chunk` out of the accepted set the threshold must exceed 0.425 —
but that *also* rejects the 40 correct unanimous answers, so τ0 collapses to 1.0
(never-accept). At α=0.10 the certificate can *tolerate* one such event and drops
to τ0=0.4 (admitting the 0.425 cluster, cost win restored, risk leaves zero); at
α=0.05 it cannot, so it routes everything to the frontier. **That single problem
is the whole tension.**

## What this makes the fan-out experiment test

The fix is not "more samples always help." It is a specific, falsifiable claim
about `scale_chunk`'s *kind* of error:

- **If the error is flaky** (the wrong output is not reliably reproduced), a higher
  tier-0 fan-out splits its cluster — some samples differ, the wrong-answer score
  drops below the ceiling, and τ0 can sit *below* the ceiling while still admitting
  the genuinely-unanimous correct answers. Tension resolved: α=0.05 certifies *with*
  a cost win.
- **If the error is confident** (every sample repeats the same wrong output), more
  samples keep the cluster unanimous, the score stays at the (higher) ceiling, and
  no fan-out separates it. Tension is structural for this problem class.

A data point pointing to *flaky*: in the earlier 3:2:1 run (tier-0 = 3 samples),
`scale_chunk` came back **correct** and scored 0.526 — consistent with the wrong
answer being one unstable draw rather than a stable misreading of the spec.

**The honest caveat that keeps this a mechanism test, not a fixed-problem test:**
every live run is a fresh sample. A 5:1:1 run may draw `scale_chunk` correct (or
wrong) regardless of fan-out, and may surface a *different* flaky cheap-tier error.
So the experiment tests whether a higher fan-out **structurally** buys threshold
headroom — separating flaky wrong answers from unanimous right ones — not whether
it "fixes `scale_chunk`."

## Recommended next run

Add `config.go-specialist-511.json` (tier0 = 5 samples, tiers 1–2 = 1 each, to
isolate the tier-0 fan-out lever at otherwise-identical cost model) and run the
same pinned command at α=0.05. Success = a valid α=0.05 certificate whose tier-0
threshold is **below 1.0** and whose cascade cost is **below always-frontier**.

## Reproduce

```bash
python3 results/analyze_tension.py results/go-specialist-211-pinned-n64.execution.json
```
