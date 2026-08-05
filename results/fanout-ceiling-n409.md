# The fan-out question is closed by a bound, not by another draw: 1.83× is the ceiling — 2026-08-05

Experiment 27. Offline, **$0**, replayed from `results/s55-fixed.records.execution.json`.

The tier-0 sample fan-out is the most-worked lever in this study: experiments 9, 10,
12a, 13, 14 and 17 turned it across **5:2:1 / 1:1:1 / 2:1:1 / 5:1:1**, with six live
draws at 5:1:1 alone. Experiment 14 bounded it as a theorem (fan-out separates *flaky*
wrong answers, never *confident* ones) and experiment 17 measured the consequence as a
frequency — 2 of 6 draws certified with a cost win. Both were at n≈53, which experiment
26 has since shown is a sample size where one observation moves the certificate.

This asks the same question at **n=409**, where it is a rate rather than a coin flip,
and computes the one quantity the earlier experiments did not: **an upper bound on what
any cheap-tier gate can ever buy.**

> **Headline: the best possible cheap-tier gate is worth 1.83×, and it is unattainable.
> The realizable gate at α ≥ 0.11 already gets 2.12× — by *exceeding* the risk an
> omniscient gate would incur, not by beating it on cost. So there is no fan-out
> setting, no statistic, and no threshold vector that turns the cascade into a
> certifiable-and-cheaper policy on this benchmark. The lever is not
> under-tuned; it is bounded, and the bound is now measured.**

## Method

```bash
python3 results/fanout_ceiling.py results/s55-fixed.records.execution.json
# and the alpha sweep, 0.365 s per point through the SHIPPED LTT:
go-cascade calibrate -from-records <usable-409>.json -alpha A -delta 0.10 -baselines
```

Every tier is profiled on every problem in the n=409 records, so any threshold vector
replays with zero model calls. Certificates come from the shipped
`calibrate -from-records`, never from a second copy of Hoeffding-Bentkus in Python
(invariant #7 lives in `Calibrate`, and its grid ordering must stay data-independent).

## Result 1 — the gate has three settings, not 121

The records profile a 2:1:1 fan-out. Tier 0's score takes **exactly three distinct
values** across all 409 problems:

| score | n | wrong | wrong-rate |
|---|---|---|---|
| 0 | 80 | 80 | 1.0000 |
| 0.1209 | 29 | 1 | 0.0345 |
| **0.4250** (unanimity ceiling) | 300 | 13 | 0.0433 |

Tier-0 accuracy is 0.7702. The LTT grid sweeps 121 thresholds, but a threshold is only
distinguishable at a value some observation crosses, so the gate has **three reachable
settings**. Reporting a grid of 121 implies a tuning freedom that does not exist, and it
is why "sweep the threshold harder" was never going to work.

## Result 2 — the confident-wrong rate, measured

The class experiment 14 proved fan-out cannot separate: unanimous at the fan-out's
Wilson ceiling (invariant #9) and refuted by execution.

**13/409 = 0.0318.** At the ceiling there are 287 correct and 13 wrong answers, all at
score **exactly 0.424987**. Any threshold that rejects the 13 rejects all 287 — they are
numerically identical, not merely close. The headroom theorem is now a measurement at
n=409 rather than an inference from a handful of events at n≈53.

## Result 3 — experiment 17's "2 of 6" was never about fan-out

Experiment 17's rule was exact across its six draws: zero confident-wrong in the clean
calibration set → τ₀ < 1 → cost win; one or more → τ₀ = 1 → no win. So P(win) is just
P(a clean set of size *m* contains none of a rate-*p* class):

| m | P(zero) | E[count] |
|---|---|---|
| 47 | 0.2191 | 1.49 |
| 53 | 0.1805 | 1.68 |
| 64 | 0.1265 | 2.03 |
| 100 | 0.0396 | 3.18 |
| 409 | 0.0000 | 13.00 |

Experiment 17 observed 2/6 = 0.333 at m≈53. Exact (Clopper-Pearson) 95% CI **[0.043,
0.777]**; the n=409 rate predicts **0.181**, comfortably inside it. So the frequency is
consistent with sampling a **fixed rate** — six more draws would re-estimate *p*, not
move it, and the fan-out setting never entered the question. This also explains the
direction of every prior result: P(win) *decreases* monotonically in n, so the cost win
was always a small-sample phenomenon and gets rarer exactly as the benchmark gets more
trustworthy.

## Result 4 — the bound (the point of the experiment)

Define the **omniscient tier-0 gate**: accept exactly the cheap-tier answers execution
says are correct, escalate the rest. It is unattainable by construction — knowing which
those are requires the answer — so it upper-bounds every fan-out, every statistic, and
every threshold vector.

| policy | $/query | risk | vs frontier |
|---|---|---|---|
| **omniscient tier-0 gate** | **$0.00337** | 0.0465 | **1.83×** |
| always-frontier | $0.00616 | 0.0587 | 1.00× |
| realizable τ=[0.1, 1] | $0.00290 | 0.0733 | 2.12× |
| realizable τ=[1, 1] | $0.01008 | 0.0587 | 0.61× |

**1.83× is the ceiling on the entire cheap-tier lever.** Note what the realizable
τ=[0.1,1] policy is doing: it is *cheaper* than omniscient (2.12× vs 1.83×) while
carrying *more* risk (0.0733 vs 0.0465). It is not beating the bound — it accepts 80
score-0 answers that are all wrong, which is cheap precisely because it is wrong. The
bound applies at matched risk, and at matched risk nothing reaches it.

This is why the cost/α tension is structural rather than a tuning failure: escalation
buys risk reduction at ~1.8× the cost, and the frontier tier's own 0.0587 risk floor is
untouchable by any routing decision (invariant #6 — the final tier has no threshold).

## Result 5 — the α sweep, and where the risk is incurred

| α | valid | τ | $/query | risk |
|---|---|---|---|---|
| 0.05, 0.07 | **false** | — | — | floor 0.0538 |
| 0.084, 0.09, 0.10 | true | [1, 1] | $0.01008 | 0.0587 |
| 0.11 – 0.20 | true | [0.1, 1] | $0.00290 | 0.0733 |

The α=0.11 discontinuity is the tension, reproduced: below it the certificate collapses
to full escalation and the cascade is **0.61× frontier, i.e. 1.6× pricier**; at or above
it the cheap tier is admitted and the cascade is 2.12× cheaper. Per-tier risk
attribution at τ=[0.1,1]: tier 0 accepts 329 with 14 wrong, the frontier tier accepts 80
with **16 wrong**. More than half the risk of the "cheap" policy is incurred at the
*frontier*, on the problems tier 0 refused — so a better cheap tier cannot fix it, which
is the same lesson experiment 26 learned by spending $3.5.

## What this establishes

1. **The cheap-tier lever is bounded at 1.83× and the bound is unattainable.** No
   fan-out setting, statistic, or threshold vector on this benchmark makes the cascade
   certifiable at a deployable α *and* cheaper than always-frontier. Six further 5:1:1
   draws would not change this; the bound does not depend on the draw.
2. **Experiment 17's 2-of-6 was small-sample sampling of a fixed 3.2% rate**, not
   evidence about fan-out (predicted 0.181, inside the exact CI on 0.333). P(win) falls
   monotonically in n, so the cost win was always an artifact of small benchmarks.
3. **The confident-wrong class is 13/409 = 0.0318 and is numerically inseparable** — 287
   correct and 13 wrong answers at score *exactly* 0.424987. Experiment 14's theorem,
   measured.
4. **The gate has three reachable settings at 2:1:1**, so a 121-point grid overstates
   the available tuning freedom.
5. **Most of the risk in the cheap policy is incurred at the frontier tier** (16 of 30
   errors at τ=[0.1,1]), which no cheap-tier intervention can address.

## What this does not establish

- **Not that a 5:1:1 fan-out has the same three-value score distribution.** These
  records are 2:1:1; a 5-sample tier 0 has a 6-value distribution and a 0.649 ceiling.
  What the bound settles is that raising the ceiling cannot help *at matched risk*,
  because the omniscient gate — which already assumes perfect separation at any
  fan-out — is only worth 1.83×. The three-settings observation is specific to 2:1:1;
  the bound is not.
- **Nothing about a different tier composition.** A cheaper *frontier* tier, or a
  fourth tier, changes the frontier cost that the bound is a ratio against. The bound
  says the cheap-tier *gate* is exhausted, not that the cost model is.
- **Not a bill.** Recorded tier cost excludes the shared spec/oracle call, which is
  ~74–91% of real spend and appears in no `Record`. These are policy ratios. See
  `results/README.md` §"A correction to every cost figure on this page".
- **Nothing about a benchmark with a different cheap-tier accuracy.** At 0.7702, 23% of
  problems must escalate no matter what. The bound is a function of that accuracy, and
  a domain where a cheap model is 0.95 would have a very different ceiling.

## Reproduce

```bash
python3 results/fanout_ceiling.py results/s55-fixed.records.execution.json
```

Records: `results/s55-fixed.records.execution.json` (488, 409 usable). No model calls,
no credentials, ~1 s.
