# Plan-once-reuse across the cascade: the third planner point closes the two-stage question — 2026-08-01

Experiments 15 ([`two-stage-arm`](two-stage-arm-2026-07-31.md)) and 16
([`two-stage-haiku`](two-stage-haiku-2026-07-31.md)) tested a **per-tier** planner
(a plan drawn inside `sampleTier`, charged to that tier, benefiting only that
tier). Both found generalist-instructs-specialist is an **accuracy lever, not a
cost lever**: an Opus planner made the cascade 3.1× pricier than always-frontier,
a Haiku planner 2.13× pricier. The one variant left untested — flagged in
`results/README.md` as "the only untested two-stage variant with a shot at
cost-positive" — was **plan-once-reuse-across-the-cascade**: draw ONE plan per
query at cascade entry and thread it into *every* tier's coder, so the single
plan charge amortises across escalation instead of being sunk at whichever tier
drew it (PR #35, `Config.PlannerModel`).

This experiment runs it live. **The result is negative, and it explains *why* in
a way the per-tier arms could not: at this fan-out the amortisation the design
was built to capture never happens, because the binding constraint is the
cheap-tier Wilson ceiling, not the plan's cost.**

## Setup

- `examples/bench/config.plan-once.json`: **cascade-level** Haiku planner
  (`claude-haiku-4-5`, $1/$5), threaded into Maverick (tier 0, $0.24/$0.97),
  Sonnet 4.5 (tier 1), Opus 4.5 (tier 2). Fan-out **2:1:1** — identical to #15/#16
  and `config.go-specialist-211.json`, so the four arms compare like-for-like.
- `test_model = claude-sonnet-4-6`, distinct from the planner and every tier
  (invariant #3 holds; 0 records contaminated).
- n = 64 (`combined.jsonl`), `-refs examples/bench -pin-api -alpha 0.05
  -delta 0.10 -compare -baselines`. Records:
  `results/plan-once-n64.{execution,judge}.json`. Live spend ~$5–7.
- The run completed without an external kill (the resume path was not needed this
  time).

## Result 1 — the plan did not even reproduce the accuracy nudge

Tier-0 (Maverick) ground-truth accuracy on the clean gated set (52 problems, 12
oracle-unsound excluded):

| arm | tier-0 planner | tier-0 true-correct |
|-----|----------------|---------------------|
| no plan (#12) | — | 0.88 |
| **plan-once (this run)** | **Haiku, cascade-level** | **0.885 (46/52)** |
| per-tier Haiku (#16) | Haiku, per-tier | 0.946 |
| per-tier Opus (#15) | Opus, per-tier | 0.92 |

Plan-once-reuse landed tier-0 accuracy at **0.885 — statistically the no-plan
baseline, and *below* both per-tier planner arms.** The two recurring confident
tier-0 errors (`scale_chunk`, `num_isqrt`) were wrong **with** the plan, matching
the #14 headroom theorem (fan-out and now plans buy nothing against a confident
error). Sonnet 0.942, Opus 0.962 — the top-tier floor is unchanged.

Why weaker than the per-tier arms? A single plan written once, before any tier
runs, is a *general* plan; the per-tier plan is drawn fresh at each tier's
temperature and (for the cheap tier) is the only thing that tier sees. The
one-plan-for-all design trades per-tier specificity for amortisation — and the
specificity, it turns out, was carrying the (already noise-band) accuracy nudge.

## Result 2 — the amortisation never happens at 2:1:1

The plan-once design pays off **only if the shared plan lets a cheaper tier
resolve the query**, so the one plan charge is spread over fewer escalations. It
does not, and the reason is structural:

- The single Haiku plan lands tier 0 at **$0.00484/query** (Maverick coder + one
  plan), 8.6× the no-plan $0.00056 — essentially identical to #16's per-tier Haiku
  (8.7×). *The plan cost is the same whether drawn per-tier or once*, because at
  2:1:1 **tier 0 is reached on every query anyway.**
- At 2 samples the cheap tier's Wilson lower bound maxes out at **0.425** (2/2
  agreement). The certified thresholds are `[0.5, 0.1]` (α≤0.10) or `[1, 1]`
  (α=0.15). **Zero of 52 clean tier-0 answers clear 0.5** — they *cannot*, the
  ceiling is below it — so every query escalates past tier 0 regardless of how
  good the plan made its answer.

So the plan is drawn once but **tier 0 is always paid and always escalated
through**: there is nothing to amortise across, because the query never *stops*
at the cheap tier. Plan-once-reuse and per-tier planning collapse to the same
cost at this fan-out. The amortisation is a fan-out property, and 2:1:1 does not
have it — the same Wilson-ceiling wall that experiments 12–14 hit on the cost
win.

## Result 3 — certification and cost, at every operating point

Execution arm, replayed offline over the saved records (free):

| α | valid | thresholds | empirical risk | cascade cost | always-frontier | verdict |
|------|-------|-----------|----------------|--------------|-----------------|---------|
| 0.05 | ✗ | [0.5, 0.1] | 0.0385 | $0.0146 | $0.0123 | no cert; **1.19× pricier** |
| 0.10 | ✗ | [0.5, 0.1] | 0.0385 | $0.0146 | $0.0123 | no cert; **1.19× pricier** |
| 0.15 | ✓ | [1, 1] | 0.0385 | $0.0243 | $0.0123 | cert, **1.98× pricier** |

- **α=0.05 does not certify** (lowest achievable empirical risk 0.0385 at n=52;
  the 12 oracle-unsound exclusions cut n below the α=0.05 sample-size floor — same
  small-n wall as #14/#16, not a plan effect).
- Where it certifies (α=0.15), the cascade is **~2× pricier** than always-frontier
  — the `[1,1]` full-escalation collapse, identical in kind to #12/#16.
- Ground-truth baselines (n=64): always-cheapest risk 0.281 / $0.0049; cascade
  0.219 / $0.0146; always-frontier 0.219 / $0.0123. **The cascade matches
  frontier's risk at higher cost at every operating point** — it never wins.

Judge arm (same candidates): lowest empirical risk 0.096, realized 0.058, α=0.05
does not certify — the execution oracle again certifies lower (0.0385 vs 0.096
empirical), consistent with every prior run.

## What this establishes

**Three planner points now close the two-stage question.** Per-tier Opus (3.1×
pricier), per-tier Haiku (2.13×), and cascade-level plan-once (≈2× at the
certified point, 1.19× elsewhere) all land on the same side: **generalist-
instructs-specialist is an accuracy lever, not a cost lever, and no plan-placement
variant reverses it.** The cheap-bottom-tier lever (experiment 11) remains the
only thing that makes the cascade beat always-frontier on cost.

The specific contribution of *this* experiment is the mechanism for **why
plan-once-reuse — the variant that looked structurally different — collapses to
the same place**: its cost advantage is conditional on the cheap tier *accepting*
(so escalations, and thus plan re-charges, are avoided), but at 2:1:1 the 2-sample
Wilson ceiling (0.425) sits below every certifiable threshold, so the cheap tier
never accepts and every query escalates through it. The amortisation the design
was built to capture is a high-fan-out property the deployable-α thresholds do not
permit. A 5:1:1 plan-once run *could* lift the ceiling (0.649, per #13) and give
the plan something to amortise across — but #14/#17 show a 5:1:1 cheap-tier
acceptance is itself draw-dependent and fragile, so this is at best a narrower,
not a general, cost win. Not pursued here.

## Honesty line

Negative result, reported as such. The accuracy comparison is on the clean gated
n=52; the 12 oracle-unsound exclusions are the recurring spec-model test-noise
class (`scale_two_sum`, `scale_fibonacci`, etc.), not model error. Risk is
sampling-noisy at n=64 and must not be compared across runs; the robust claims
here are the **cost ordering within this run** (cascade ≥ frontier at every α) and
the **structural amortisation argument** (draw-independent, from the Wilson
ceiling). No mock numbers are cited.
