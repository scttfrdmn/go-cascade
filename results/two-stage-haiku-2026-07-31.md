# Cheap-planner two-stage (Haiku plans, Maverick codes): a cheaper planner mitigates but does not reverse the cost penalty — 2026-07-31

Experiment 15 ran the two-stage tier with the *most expensive* possible planner
(Opus-4.5, = the top tier) and found generalist-instructs-specialist to be an
**accuracy lever, not a cost lever**: the plan nudged tier-0 accuracy up slightly but
an Opus planner call on every cheap-tier query made the cascade **3.1× pricier than
always-frontier**. That writeup flagged the obvious follow-up — a *cheap* planner
(Haiku/Nova) is "the only pairing with a chance of net cost benefit."

This is that run. Pairing: **Haiku-4.5 plans, Llama-4-Maverick codes**, 2:1:1,
`-pin-api`, `-compare`, n=64 — *identical* to experiment 15 except the planner is
Haiku-4.5 ($1/$5 list) instead of Opus-4.5 ($5/$25). Haiku is ~4× cheaper per token
than Opus, so this is the best realistic case for the two-stage economics with a
Claude planner. **Result: a cheaper planner does mitigate the penalty (2.13× pricier
vs 3.1× with Opus), and it now certifies at α=0.05 — but it does not reverse it.**
Every tier-0 answer still amortizes a planner call it cannot earn back, and at the
one operating point where tier-0 acceptance fires the cascade is barely cheaper than
frontier (1.17×) versus the non-planned cascade's 5.24×.

## Result 1 — the plan nudges tier-0 accuracy up, same direction as Opus

Tier-0 (Maverick) correctness with the Haiku plan, against experiment 12's identical
coder/fan-out with no planner and experiment 15's Opus plan:

| | tier-0 true-correct |
|--------------------|---------------------|
| two-stage (Opus plan, #15) | 47/51 = 0.92 |
| **two-stage (Haiku plan, this run)** | **53/56 = 0.946** |
| single-stage (no plan, #12) | 46/52 = 0.88 |

The Haiku plan lifts tier-0 ground-truth accuracy to **0.946** — as good as (a hair
above) the Opus plan, and above the no-plan baseline. But `scale_chunk` — the
confident-wrong problem from experiments 12–15 — **stayed confident-wrong with the
Haiku plan too** (still oracle-refuted at tier 0), joined by `num_isqrt` and
`str_first_unique`. The plan does not fix Maverick's reliably-repeated mistakes,
regardless of who writes it — consistent with the experiment-14 theorem: a confident
error is a confident error regardless of upstream help.

(β=0 held: execution realized risk equals empirical, 0/0. The `-compare` judge arm
again over-rejected — empirical 0.085 vs realized 0.000 — the §5.5 η_fa gap.)

## Result 2 — a cheap planner shrinks the penalty from 3.1× to 2.13×, but it is still a penalty

Ground-truth cost baselines (n=64, all records):

| policy | risk | mean cost/query |
|--------|------|-----------------|
| always-cheapest (tier 0 only) | 0.172 | $0.00487 |
| **cascade (two-stage, α≤0.10)** | 0.078 | **$0.01835** |
| always-frontier | 0.078 | $0.00849 |

The cascade is **2.13× more expensive than always-frontier** at the certified
thresholds — a real improvement over experiment 15's 3.1×, but the *same sign*. The
cause is unchanged, only smaller: the Haiku planner runs on *every* tier-0 query, so
mean tier-0 cost is **$0.00487 — 8.7× the $0.00056** of unplanned Maverick (experiment
15's Opus planner was 35×). A cheaper planner shrinks the per-query overhead, but the
structure is identical: the cascade pays a planner call at tier 0 and then, on
escalation, pays the frontier anyway.

**Where tier-0 acceptance fires, the penalty is only partly hidden.** At α=0.05 and
α=0.10 the certified thresholds collapse to `[1,1]` (full escalation — the 2-sample
Wilson ceiling cannot clear a strict threshold, the experiment-12 tension), so the
cascade is pure dead-weight planner+coder spend on top of frontier escalation. Only at
α=0.15 does tier-0 acceptance fire (`[0.1,1]`):

| α | thresholds | cascade risk | cascade cost | vs frontier |
|------|------------|--------------|--------------|-------------|
| 0.05 | [1, 1] | 0.000 | $0.01808 | **2.13× (pricier)** |
| 0.10 | [1, 1] | 0.000 | $0.01808 | **2.13× (pricier)** |
| 0.15 | [0.1, 1] | 0.109 | $0.00728 | **1.17× cheaper** |

Compare the non-planned #12 cascade at the same α=0.15: `[0.1,1]`, **5.24× cheaper**
($0.00152). The Haiku plan turns a 5.24× cheap-tier win into a 1.17× squeaker — the
planner call eats almost the entire cheap-tier cost advantage even at the operating
point designed to exploit it.

## What this establishes

1. **A cheaper planner is a real mitigation, not a fix.** Haiku-4.5 halves the tier-0
   overhead versus Opus (8.7× vs 35× the unplanned cost) and the α=0.05 cert now
   *passes*, but the cascade is still 2.13× pricier than always-frontier at the
   certified thresholds and only 1.17× cheaper where tier-0 acceptance fires. **With
   two planner points now on the curve (Opus 3.1×, Haiku 2.13×), the direction is
   confirmed: generalist-instructs-specialist is an accuracy lever, not a cost lever.**
   The cheap-bottom-tier lever (experiment 11 / non-planned #12) beats it at every
   operating point.
2. **The accuracy lift replicates but still does not touch confident errors.** Tier-0
   ground-truth accuracy 0.946 (Haiku) ≈ 0.92 (Opus) > 0.88 (no plan) — the plan helps
   a little regardless of planner, but `scale_chunk` stayed confident-wrong under both
   planners, matching the experiment-14 theorem.
3. **α=0.05 now certifies — because this draw had no top-tier miss, not because of the
   planner.** Experiment 15 was valid=false at α=0.05 on an Opus (top-tier) miss on
   `hard_num_mean_overflow`; this run's top tier got it right, so the cert passes. That
   is the experiment-14 small-n wall (one error anywhere moves the certificate), and it
   is orthogonal to the two-stage mechanism — do not read the passing cert as a
   two-stage win.

## The remaining untested variant

The only structural change left that could plausibly make a two-stage tier
cost-positive is **plan once, reuse across the whole cascade** — amortise a single
planner call over all tiers instead of charging it only at tier 0, so escalation does
not pay for planning twice. That is a code change (the plan currently lives in
`sampleTier` and is regenerated per tier fan-out), not a config. Even then the honest
read stands: the cheap-bottom-tier cost win (experiment 11) is the stronger lever, and
a two-stage tier's value is at best an accuracy top-up whose planner cost must be
smaller than the escalation it prevents — which, at 2.13× even with Haiku, it is not
here.

## Honest limitations

- **One pairing, one draw, small n.** The +0.06 accuracy over no-plan (0.88→0.946) is
  noise-band at n≈56; the robust findings are the *cost sign* (still pricier, from a
  structural per-query planner charge) and the *shrinkage* from Opus to Haiku (35×→8.7×
  tier-0 overhead is structural, not noise).
- **Cross-run risk is not comparable** (fresh draws). The tier-0 accuracy comparison is
  the like-for-like signal (same coder, same fan-out, same benchmark as #12/#15); the
  cost comparison is within-run.
- **Haiku is priced at its $1/$5 list approximation**, the same convention every prior
  Claude tier uses (only Maverick carries a verified real Bedrock rate). A genuinely
  near-free planner (Nova Micro) is still untested and unpriced in this repo — but the
  structural conclusion (a per-tier-0 planner charge you cannot earn back) holds for any
  non-trivial planner price.
- **Certification passing here is a clean-draw artifact**, orthogonal to the two-stage
  mechanism — as in #15, not evidence for or against the arm.

## Reproduce

```bash
# Live (needs Bedrock; ~$4-6 with -compare + the Haiku planner). If SIGTERM'd, +-resume:
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.two-stage-haiku.json \
  -bench examples/bench/combined.jsonl -refs examples/bench -pin-api \
  -alpha 0.05 -delta 0.10 -compare -baselines \
  -records results/two-stage-haiku-maverick-n64.execution.json

# Offline cost/accuracy comparison at each operating point (free):
for a in 0.05 0.10 0.15; do
  go-cascade calibrate -from-records results/two-stage-haiku-maverick-n64.execution.json \
    -alpha $a -delta 0.10 -baselines -o /tmp/h-$a.json
done
```

Committed: `two-stage-haiku-maverick-n64.{execution,judge}.json`. Live spend this
experiment: recorded tier calls + Haiku planner calls + spec/pin overhead, paired with
`-compare` (well within the ~$4-6 scope; cheaper than #15's Opus-planner ~$4.6).
