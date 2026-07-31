# Two-stage tier (generalist-instructs-specialist): an accuracy lever, not a cost lever — 2026-07-31

The last untested lever in the study: a strong **planner** model rewrites each
problem into an implementation plan, and a cheap **coder** model generates from that
plan instead of the raw problem (the two-stage tier shipped in PR #28). The
hypothesis (README "two candidate levers"): a good plan lets a cheap coder punch
above its single-stage weight, buying tier-0 accuracy without escalating.

This is the first live run. Pairing: **Opus-4.5 plans, Llama-4-Maverick codes**,
2:1:1, `-pin-api`, `-compare`, n=64. It is directly comparable to experiment 12's
`config.go-specialist-211.json` — *identical* Maverick coder and fan-out, the only
difference being the added Opus planner call. The result is a **negative result with
a clear signal**: the plan helps accuracy slightly, but at an Opus planner the
economics are inverted — the cascade becomes **3.1× more expensive than
always-frontier**.

## Result 1 — the plan nudges tier-0 accuracy up, barely

Tier-0 (Maverick) correctness, with the Opus plan vs without (experiment 12's
identical coder/fan-out, no planner):

| | tier-0 oracle-correct | tier-0 true-correct |
|--------------------|-----------------------|---------------------|
| **two-stage (Opus plan)** | 47/51 = **0.92** | 47/51 = 0.92 |
| single-stage (no plan) | 46/52 = 0.88 | 46/52 = 0.88 |

A **+0.04** lift — one problem, at n≈51, well within sampling noise. Not the
transformation the hypothesis hoped for. And `scale_chunk` — the confident-wrong
problem from experiments 12–14 — stayed **confident-wrong even with the plan** (still
at the unanimity ceiling, oracle-refuted). The plan did not fix Maverick's
reliably-repeated mistake, consistent with the experiment-14 theorem: a confident
error is a confident error regardless of upstream help.

(β=0 held: execution realized risk equals empirical, as always. The `-compare` judge
arm again over-rejected — empirical 0.078 vs realized 0.039 — the §5.5 η_fa gap.)

## Result 2 — the Opus planner makes the cheap tier as expensive as the frontier

Ground-truth cost baselines (n=64, all records):

| policy | risk | mean cost/query |
|--------|------|-----------------|
| always-cheapest | 0.266 | $0.01975 |
| **cascade (two-stage)** | 0.203 | **$0.02669** |
| always-frontier | 0.219 | $0.00851 |

The cascade is **3.1× more expensive than always-frontier**. The cause is direct:
the Opus planner call runs on *every* tier-0 query, so mean tier-0 cost is
**$0.01975 — 35× the $0.00056** of unplanned Maverick, and even "always-cheapest"
(tier 0 only) now costs more than always-frontier. Putting a frontier-priced call in
front of the cheap tier **defeats the entire purpose of a cheap bottom tier**: the
cascade pays frontier prices at tier 0 and then, on escalation, pays them again.

## What this establishes

1. **Generalist-instructs-specialist is an accuracy lever, not a cost lever — and at
   an Opus planner it is a cost disaster.** The mechanism works (the plan reaches the
   coder; tier-0 accuracy rises a little), but a frontier-priced planner on every
   cheap-tier query inverts the economics: 3.1× *more* expensive than just always
   calling the frontier. This is the opposite of experiment 11's cheap-bottom-tier
   cost win.
2. **The accuracy lift is small and does not touch confident errors.** +0.04 tier-0
   accuracy (one problem, noise-band), and `scale_chunk` stayed confident-wrong with
   the plan — matching the experiment-14 theorem that no upstream help separates a
   confidently-repeated wrong answer.
3. **Certification failed on a top-tier miss, not the mechanism.** valid=false at
   α=0.05 because Opus (top tier) got `hard_num_mean_overflow` wrong — the same
   accuracy-at-small-n wall as experiment-14 draw-c, unrelated to the two-stage tier.

## The economically plausible version (untested)

For a two-stage tier to make cost sense, the planner must be **much cheaper than the
frontier it is trying to avoid escalating to** — otherwise the plan costs more than
the escalation it prevents. This run used the most expensive possible planner (Opus,
= the top tier), so it is the *worst case* for the economics and the *best case* for
plan quality. The natural follow-ups, if the arm is pursued:

- **A cheap planner** (Sonnet, or a mid-size non-frontier model): would the smaller
  accuracy lift still beat a much smaller planner cost? Sonnet at $3/$15 vs Opus at
  $5/$25 is only ~1.7× cheaper — likely still cost-negative. A genuinely cheap
  planner (Haiku, Nova) is the only pairing with a chance of net cost benefit.
- **Plan once, reuse across the whole cascade** (not just tier 0): amortise the
  planner cost over all tiers rather than charging it only at tier 0. Structural
  change, not just config.

Neither is obviously a winner; the honest read is that **the cheap-bottom-tier cost
win (experiment 11) is a stronger lever than instructing a specialist**, and the
two-stage arm's value is at best an accuracy top-up whose cost must be tightly
controlled.

## Honest limitations

- **One pairing, one draw, small n.** +0.04 accuracy is noise-band; do not read it as
  "the plan helps by 4 points." The robust finding is the *cost inversion* (35× tier-0
  cost blowup is structural, not noise) and the *direction* of the accuracy effect.
- **Opus is the worst case for economics / best for plan quality.** A cheaper planner
  is untested; the arm is not dead, but this pairing is clearly not the answer.
- **Cross-run risk is not comparable** (fresh draws). The tier-0 accuracy comparison
  is the like-for-like signal (same coder, same fan-out, same benchmark); the cost
  comparison is within-run.
- **Certification failure is a top-tier accuracy miss**, orthogonal to the two-stage
  mechanism — not evidence against (or for) the arm.

## Reproduce

```bash
# Live (needs Bedrock; ~$8-12 with -compare + the Opus planner). If SIGTERM'd, +-resume:
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.two-stage.json \
  -bench examples/bench/combined.jsonl -refs examples/bench -pin-api \
  -alpha 0.05 -delta 0.10 -compare -baselines \
  -records results/two-stage-opus-maverick-n64.json

# Offline cost/accuracy comparison vs the no-planner run (free):
go-cascade calibrate -from-records results/two-stage-opus-maverick-n64.execution.json -alpha 0.05 -delta 0.10 -baselines -o /tmp/c.json
```

Committed: `two-stage-opus-maverick-n64.{execution,judge,cert}.json`. Live spend this
experiment: ~$4.6 recorded tier calls + Opus planner calls + spec/pin (paired with
`-compare`).
