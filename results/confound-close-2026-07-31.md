# Closing experiment 13's confound: the α=0.05 cost win is real but fragile — 2026-07-31

Experiment 13 ([`fanout-511`](fanout-511-2026-07-30.md)) reported that a 5:1:1
fan-out delivered **deployable α=0.05 certification AND the cost win, together** —
but flagged a confound: its headline draw happened to put the 2:1:1 blocker
(`scale_chunk`) *correct*, so structural headroom and a lucky draw could not be
separated. This experiment closes that confound two ways: an analytical theorem
(draw-independent) and two fresh repeat draws (empirical). The result **revises
experiment 13's headline**: the cost win at α=0.05 is real but **does not
replicate** — it is fragile to a single cheap-tier or top-tier error at n≈53.

## Part 1 — the headroom theorem (analytical, free)

`results/headroom_theorem.py` derives, from the Wilson-LCB routing rule alone (no
model, no spend), why a higher fan-out helps against a *flaky* wrong answer and not
a *confident* one. An answer reproduced with per-sample probability p forms a
cluster of size k~Binomial(N,p); the tier is accepted iff wilsonLCB(k,N) ≥ τ.
Sweeping every achievable threshold, the best discrimination gap between a correct
answer (p=0.9) and a wrong one:

| fan-out N | vs FLAKY wrong (p=0.5) | vs CONFIDENT wrong (p=0.9) |
|-----------|------------------------|-----------------------------|
| 1 | 0.40 | 0.00 |
| 2 | 0.56 | 0.00 |
| 5 | 0.73 | 0.00 |
| 10 | 0.88 | 0.00 |

The gap **widens with N against flaky errors** and is **exactly 0 for all N against
confident errors**. Fan-out buys discrimination headroom against flakiness and none
against a wrong answer the model reproduces as reliably as a right one. This is the
tension-diagnosis dichotomy, proven from the scoring rule.

*(This also corrects a loose framing in experiment 13: at a **fixed** threshold more
samples do not monotonically reject flaky-wrong — the required agreement fraction
k*/N falls as N grows. The benefit is the best achievable discrimination, not
fixed-threshold rejection.)*

## Part 2 — three fresh 5:1:1 draws (empirical): the win does not replicate

Three independent 5:1:1 draws (same config, `-pin-api`, α=0.05, n=53 clean each;
draws b/c without `-compare` to halve cost), and **three different outcomes**:

| draw | certifies α=0.05? | thresholds | what happened |
|------|-------------------|-----------|---------------|
| **a** (exp 13) | **yes** | **[0.4, 1]** | clean — cheap tier accepted, **cost win** |
| **b** | yes | **[1, 1]** | a cheap-tier **confident-wrong** (`scale_is_palindrome`, 5/5 agreement) forces full escalation — **no cost win** |
| **c** | **no** (valid=false) | [0.3, 0] | a **top-tier miss** (`hard_num_mean_overflow`, Opus wrong); fixed-sequence halts at `[1,1]` (p=0.50 at n=53) — **certificate invalid** |

Three draws, three distinct failure modes:

- **draw-a** is the experiment-13 result: no confident cheap-tier error and no
  top-tier miss this draw, so τ0=0.4 certifies and the cascade is 2.13× cheaper than
  frontier at 0 risk.
- **draw-b** reverts to τ0=1.0. `scale_is_palindrome` — a problem `scale_chunk`
  didn't warn us about — came back **confident-wrong**: all 5 Maverick samples agreed
  on the same wrong output (score 0.649, the unanimity ceiling). Per the Part 1
  theorem, no fan-out separates that from a correct unanimous answer, so τ0 collapses
  to 1.0 and the cost win is gone. (`scale_chunk` this draw was *flaky*, 3/5 → 0.272,
  and escalated harmlessly.)
- **draw-c** does not certify at all. Opus (the top tier) got `hard_num_mean_overflow`
  wrong, so the fully-escalated policy `[1,1]` carries risk 1/53=0.019, whose
  Hoeffding-Bentkus p-value (0.50) exceeds δ=0.10 at n=53. Fixed-sequence
  (invariant #7) tests `[1,1]` first and halts → valid=false. A risk-0 vector
  *does* exist (`[0.3,0]`, which accepts at the mid tier where Opus erred — escalation
  is not monotone in correctness), but the data-independent ordering must clear
  `[1,1]` before reaching it, so it is unreachable. This is a **top-tier accuracy**
  failure — even always-frontier (= `[1,1]`) would not certify this draw — unrelated
  to the cheap tier or fan-out.

## What this establishes

1. **The headroom theorem is solid and draw-independent:** fan-out helps against
   flaky cheap-tier errors, never against confident ones. Experiment 13's mechanism
   is correct.
2. **But the α=0.05 cost win does NOT replicate.** Only **1 of 3** fresh draws got
   τ0<1.0. Experiment 13's "deployable α *and* cost win, together" was a
   **single-draw result**; it is fragile, not robust.
3. **At n≈53 a single error anywhere moves the certificate.** A confident cheap-tier
   error kills the cost win (draw-b); a top-tier miss kills certification entirely
   (draw-c). The binding constraint at this benchmark size is **model accuracy at
   small n** — the same wall experiment 8 hit, now seen from the fan-out side.
4. **The cheap tier stayed safe in all three draws:** 0 tier-0 acceptance-risk events
   throughout (nothing the oracle accepted at tier 0 was truly wrong). The failures
   are *escalation-forcing* (cost) or *top-tier* (accuracy), never cheap-tier
   over-acceptance.

## Honest limitations

- **Three draws is still tiny.** "1 of 3" is not a rate; it says the win is not
  reliable, not how often it occurs. A dozen draws would estimate the frequency.
- **Draws b and c ran without `-compare`, so `true_correct` (execution ground truth)
  is not populated** — these two draws carry only the oracle verdict. The confound
  question (τ0, flaky/confident split) needs only `score` + oracle `correct`, which
  are present, so the certification/cost conclusions hold. But I cannot independently
  re-confirm on these two draws that the oracle was sound on `scale_is_palindrome` /
  `hard_num_mean_overflow` the way a `-compare` run's β=0 check does. Both problems
  passed the `-refs -pin-api` gate (reference compiled and passed the generated
  tests), so the errors are most likely genuine model errors, not oracle noise — but
  that is an inference, not a measured β=0 on these draws.
- **`scale_is_palindrome` / `hard_num_mean_overflow` are single occurrences.** Whether
  Maverick is *reliably* confident-wrong on the palindrome spec (vs a one-draw fluke)
  is unmeasured; the theorem only says *if* it is confident-wrong, fan-out won't help.

## What this means for the study

Experiment 13's cost-win-at-deployable-α should be read as **"achievable on a good
draw, not guaranteed."** The robust, replicated claims remain: (a) the executable
oracle is sound and certifies where the judge cannot; (b) fan-out buys headroom
against flaky cheap-tier errors (theorem + observed splits); (c) the cheap tier does
not over-accept. What does **not** hold is that 5:1:1 *reliably* buys both deployable
α and the cost win — that took a lucky draw, and at n≈53 a single confident cheap
error or top-tier miss is enough to break it. Reaching a *robust* deployable-α cost
win needs either more calibration data (to survive one error) or a more accurate
cheap tier (fewer confident-wrong answers) — not more fan-out.

## Reproduce

```bash
python3 results/headroom_theorem.py    # the analytical dichotomy, free
python3 results/analyze_draws.py \
  '5:1:1-a=results/go-specialist-511-pinned-n64.execution.json' \
  '5:1:1-b=results/go-specialist-511-draw2.json' \
  '5:1:1-c=results/go-specialist-511-draw3.json'

# Per-draw certificates (offline):
go-cascade calibrate -from-records results/go-specialist-511-draw2.json -alpha 0.05 -delta 0.10 -o /tmp/b.json  # valid, [1 1]
go-cascade calibrate -from-records results/go-specialist-511-draw3.json -alpha 0.05 -delta 0.10 -o /tmp/c.json  # invalid
```

Live spend this experiment: ~$2 recorded tier calls (two single-arm draws) + spec/pin
generation; no `-compare`, so roughly half a paired run each.
