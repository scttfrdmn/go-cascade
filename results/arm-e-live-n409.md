# Experiment 24 — §5.5(2) arm (e) live: self-consistency at matched cost loses to execution

**Run:** `go-cascade selfconsistency -sample`, tier 0 (Llama 4 Maverick), τ=[1,1], n=409
usable of 488 MultiPL-E Go problems. 488/488 attempted, no kill.
**Records:** `results/arm-e-n409.json`.
**Bill:** $18.77 total — $4.71 matched sampling, $14.06 shared oracle.

Experiment 23 established for $0 that arm (e) is only well-posed at the cheap tier (a
frontier arm at matched cost is always-frontier relabelled on 99.5% of problems). This is
the paid pass at the one tier that survives that check.

## Headline

At matched per-problem cost, on **identical candidates**, a source-text majority vote
selects a correct program less often than behavioural clustering:

| selector | correct | rate |
|---|---|---|
| arm (e), text vote | 295/366 | **0.806** |
| §3.5, behavioural cluster | 335/366 | **0.915** |

Paired on the 366 rows where both selectors answered. Median fan-out **50** samples
(p10–p90 39–86, mean 57.8) — so the text vote is getting roughly 50 votes on how the code
is *written* against 2 on what it *does*, for the same money, and still loses.

McNemar on the discordant pairs, which is what a paired design actually reads:

- text wrong / cluster right: **45**
- text right / cluster wrong: **5**
- exact two-sided binomial **p = 4.2 × 10⁻⁹**

A 9:1 asymmetry. This is the arm §3.5 is argued *against*, and it was given every
advantage: raw plurality mass rather than a Wilson bound (invariant #9 governs the routing
score crossing a calibrated threshold; arm (e) crosses none), per-problem budget matching
rather than mean matching, normalised source so formatting and comments cannot split
agreeing candidates, and no access to the verifier when picking its winner.

## Read the denominators before the rates

**The run's first printed summary was misleading and the code has been fixed.** It showed
`text 296/409 (0.724)` against `cluster 335/366 (0.915)` — a 0.19 gap. Both numbers were
individually correct, but they have **different denominators**: the cluster arm's 43
abstentions are properly excluded, while the text arm has no abstention concept and so was
divided by all 409. The paired gap is **0.11**, not 0.19.

Nothing failed, because nothing was wrong — the report just invited a comparison the data
does not support. `printSelfConsistencySummary` now leads with the paired line on one
denominator, prints the discordant counts, gives the abstention rows their own rate, and
labels the all-rows text figure explicitly incomparable.
`TestSelfConsistencySummaryPairsOnOneDenominator` asserts the *shape* of the report rather
than any value, since a value assertion would have passed on the misleading version.

This is the same hazard as the `-baselines`-over-488 vs `analyze_s55.py`-over-409 split
already documented for experiment 21. Always quote the denominator.

## The sharpest result is in the abstentions

Where the behavioural cluster abstained — nothing survived the ladder, which is a sound
refutation of the whole sample (invariant #4) and an escalation in a real cascade — the
text vote was correct **1/43 (0.023)**.

And it was not hedging. Its mean vote mass on those rows was **0.604** against **0.661**
elsewhere: a 0.06 drop, while its accuracy fell from 0.806 to 0.023. **The text vote is
confidently wrong exactly where execution knows it has nothing.** Agreement among samples
carries almost no signal about correctness in the region where correctness is hardest;
execution does, and it reports the absence of a survivor rather than electing one.

That asymmetry is the §3.5 argument in one number, and it is not what a "self-consistency
is a weaker selector" framing would predict — a weaker selector should degrade gradually,
not invert. It is also *why* the abstention must never be scored as a wrong answer: doing
so would flatter the cluster arm (0.819 over 409) while hiding the finding that makes the
comparison interesting.

## Agreement is uninformative; disagreement is everything

| | n | text right | cluster right |
|---|---|---|---|
| selectors agreed | 313 | 293 | 290 |
| selectors disagreed | 53 | **2** | **45** |

On the 313 rows where the two selectors picked equivalent programs they are equally
accurate (0.936 vs 0.927). The entire margin lives in the 53 disagreements, where the
cluster is right 45 times and the text vote twice (6 rows neither). So the result is not
"text voting is noisy" — it is that **when the two selectors diverge, the divergence is
almost always the text vote preferring a popular wrong program.** Popularity among samples
and correctness come apart precisely where they matter.

## Cost, and a note on what "matched" excludes

| term | amount |
|---|---|
| matched sampling (the budget arm (b) spent) | $4.7086 |
| overshoot above the matched budget | +$0.6412 (15.55%), 376/409 rows |
| shared oracle: spec + plan, excluded from the match by construction | $14.0574 |
| **total bill** | **$18.7660** |

The overshoot is now recorded per row (`OverBudgetUSD`) rather than invisible. Its cause is
per-batch pricing error: a batch's true per-sample cost is only known once it returns, so a
short probe both buys a larger fan-out and underprices it, compounding upward. Bounded by
roughly one batch's estimation error, but a reader is entitled to see it.

The oracle is **75% of the bill** and is excluded from the *match* because arm (b)'s
recorded spend excludes it too — the comparison would be unfair otherwise. It is not
excluded from the *invoice*. This is the same term that makes every records-derived cost
figure in this directory an understatement; see "A correction to every cost figure on this
page" in `README.md`. The originally approved figure for this run, **$4.12**, was the
matched budget quoted as an invoice — off by 4.6×.

## What this does not show

- **One tier.** Arm (e) ran only at tier 0, because experiment 23 ruled the other two out
  as degenerate at matched cost. So this is a statement about self-consistency *at a cheap
  model's fan-out*, which is the only place the comparison is well-posed — not about
  self-consistency at frontier quality.
- **No certificate.** Arm (e) crosses no calibrated threshold, so nothing here is
  certified and no α is claimed. It is a selector comparison, not a risk result.
- **The oracle is the generated suite**, pinned via `-refs` to the same contract the
  profiled records used. That was a fix made before this run: the paid path originally
  never called `SetPinnedAPIs`, which would have scored arm (e) against a differently
  named, soundness-unchecked suite while arm (b) used a validated one.
- **MultiPL-E Go has 0 concurrency problems**, so the `-race` rung never fired here
  either. The text vote's blind spot for reader-invisible defects is the class most likely
  to widen this gap, and it is the class this benchmark cannot exhibit (issue #50).

## Reproduce

```bash
# The free feasibility check that gates the paid pass (mock only builds the router):
go-cascade selfconsistency -records results/s55-fixed.records.execution.json \
  -config examples/bench/config.go-specialist-211.json -tau 1,1 -provider=mock

# The paid pass. -refs is REQUIRED: it pins each problem's API to the contract the
# profiled records were built against, so both arms share one oracle.
AWS_PROFILE=aws go-cascade selfconsistency -sample -tier 0 \
  -records results/s55-fixed.records.execution.json \
  -config examples/bench/config.go-specialist-211.json \
  -refs ~/mple-bench/refs -tau 1,1 -resume -o results/arm-e-n409.json
```
