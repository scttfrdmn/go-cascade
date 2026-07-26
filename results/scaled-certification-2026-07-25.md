# Scaled certification comparison (n=64) — 2026-07-25

The certification comparison at n=28 (`certification-comparison-2026-07-25.md`)
showed the executable oracle certifies a lower risk bound than the judge, but the
best certifiable α was ~0.27 — far above a deployable ≤0.05–0.10. That ceiling
was blamed on sample size (α=0.05 needs n≥45, paper eq. 7). This run tests that
diagnosis: a combined **64-problem** benchmark (`examples/bench/combined.jsonl` =
28 pilot + 12 hard + 24 scale), paired `--compare`, δ=0.10, zero skips.

## Result

| oracle    | empirical risk | lowest certifiable α (δ=0.10) |
|-----------|----------------|-------------------------------|
| execution | 0.109          | **0.19**                      |
| judge     | 0.188          | **0.30**                      |

Compared to n=28 (execution 0.27, judge 0.32):

- **The comparative gap widened from 5 to 11 points** (0.19 vs 0.30). More data
  sharpened the executable oracle's advantage — the direction the theory predicts,
  now at a defensible n. The judge's empirical risk (0.19) is ~1.7× execution's
  (0.11), again driven by over-rejection (β), not by passing wrong code.
- **Execution's certifiable α fell from 0.27 to 0.19** as n grew — scaling did
  tighten the bound, as intended.

## The honest finding: the ceiling is model accuracy, not sample size

α=0.05 remained **unreachable**, and the reason is now clear and measured.
Execution's empirical risk is **0.109**, essentially identical to n=28's 0.107.
The certifiable α can never drop below the empirical risk (you cannot certify a
risk lower than the error rate you actually observe), so with ~11% true error the
floor is ~0.11–0.19 regardless of how large n grows. Scaling n moved the
certifiable α *toward* that floor (0.27→0.19); it cannot move it *below* it.

So the earlier "raise n" diagnosis was half right. Two things gate a deployable
α, and they are different levers:

1. **Sample size** gated the n=28 result (0.27 was well above the 0.11 floor).
   Fixed here: n=64 reached 0.19, close to the floor.
2. **Model accuracy** gates from here down. The remaining gap (0.19 vs a
   deployable 0.05) is not statistics — it is that the cascade's true error rate
   on this benchmark is ~11%. Reaching α=0.05 requires the *models* (or the
   cascade's escalation/repair) to be more correct, not more calibration data.

Where the ~11% comes from: the cheapest tier's accepted solutions are wrong on
about 1 in 15 problems, and escalation recovers many but not all of the rest.
Driving the floor down means better escalation, more tiers, or a stronger top
tier — an accuracy problem, not a certification one.

## What this establishes

- **The executable oracle certifies a strictly and increasingly lower bound than
  the judge** (11-point gap at n=64, up from 5 at n=28), on identical candidates.
  This is the paper's primary §5.5 comparative claim, now demonstrated at n=64.
- **Soundness held at scale**: execution's realized risk = empirical risk = 0.109
  across 64 problems (β=0).
- **The binding constraint on deployable α is model accuracy, not sample size** —
  a measured result, not an assumption, and one the earlier smaller runs could not
  separate.

## What is still NOT established

- **No deployable α (≤0.05).** Blocked by the ~11% accuracy floor, not by n. This
  needs a more accurate cascade, not more data.
- **No cost baseline.** This measures *which risk each oracle can certify*, not
  *whether the cascade is cheaper than always using the frontier model at equal
  correctness* — still the project's central unrun experiment (README "known
  gaps"). Per-run cost is recorded, but the head-to-head vs. always-frontier has
  not been run.
- **One benchmark, one judge model, one paired run.** Directions are robust;
  exact α values are point estimates.

## Reproduce (offline, free)

```bash
for a in 0.05 0.10 0.19 0.30; do
  go-cascade calibrate -from-records results/scaled.execution.json -alpha $a -delta 0.10 -o /tmp/e.json | grep -E 'valid|empirical'
  go-cascade calibrate -from-records results/scaled.judge.json      -alpha $a -delta 0.10 -o /tmp/j.json | grep -E 'valid|empirical'
done
```

Live spend this run: ~$6 (64 problems, paired). Session total: roughly $29–33.
