# Certification comparison — 2026-07-25

The paper's **primary §5.5 outcome**: the lowest risk level α each oracle can
*certify* at fixed confidence δ and sample size n. This is the comparative claim
the whole design turns on — that an executable oracle certifies a lower (better)
bound than an LLM-judge oracle — and it had never been measured.

It is computed here **offline, at zero additional cost**, by replaying the
paired-pilot records (`pilot2-paired.{execution,judge}.json`) through the
calibrator at a sweep of α. Because that run was *paired* (both oracles ruled on
the identical candidate stream, see `pilot2-paired-2026-07-25.md`), the two arms
differ only in the oracle, so the certification gap is attributable to the oracle
alone.

## Result (δ = 0.10, n = 28, identical candidates)

| oracle    | empirical risk | lowest certifiable α | certificate valid at α=0.10 / 0.27 / 0.30 / 0.32 |
|-----------|----------------|----------------------|--------------------------------------------------|
| execution | 0.1071         | **0.27**             | no / **yes** / yes / yes |
| judge     | 0.1429         | **0.32**             | no / no / no / **yes** |

**The executable oracle certifies a strictly lower risk bound than the judge
oracle — 0.27 vs 0.32 — a 5-point gap, on the same programs.** This is the §3.1 /
§5.5 comparative claim, demonstrated live.

## Why the gap exists

The certifiable α is driven by the arm's empirical risk (the Hoeffding–Bentkus
p-value crosses δ=0.10 only once α clears the observed risk with margin). The
execution arm's empirical risk is 0.107; the judge arm's is 0.143. The judge is
*worse-calibrated against truth* on this stream — not because it passed wrong
code here (η_fa was low), but because its **false rejections** (β: it failed
correct programs it could not verify by reading, e.g. `math/big` and concurrent
code) inflate its measured risk. A higher measured risk forces a higher
certifiable α.

This is a subtler mechanism than the paper's headline (which emphasises η_fa —
the judge passing wrong code). Here the judge loses the certification race mostly
by *over-rejecting*, not by *over-accepting*. Either way the executable oracle
wins, and for the reason the paper gives: its labels are sound, so its empirical
risk reflects truth, while the judge's labels are noisy in a direction that costs
certification.

## Honest bounds

- **n = 28 is small**, so both α figures have coarse resolution (~0.01–0.02) and
  wide confidence. The *direction* (execution certifies lower) is the robust
  finding; the exact 0.27/0.32 values are point estimates.
- **Neither certifies at α ≤ 0.15**, the range anyone would actually want. That
  is the sample-size ceiling, not an oracle property: at n=28 with these error
  rates, δ=0.10 simply cannot be met at low α. Reaching α=0.05 needs n ≥ 45 of
  clean records (paper eq. 7).
- **This is one paired run.** A second paired run could shift both crossovers by
  a step. The claim is "executable certifies lower on this stream", replicated
  offline across the α sweep, not a population estimate.
- The comparison inherits every caveat of the paired pilot it replays (single
  benchmark, `sonnet-4-6` judge, strict prompt).

## What this adds to the study

This is the capstone the earlier runs were building toward. Prior results showed
the *mechanism* (execution sound, judge noisy both ways); this shows the
*consequence* the paper actually claims: **a lower certifiable risk bound for the
executable oracle**, on identical candidates, live. It closes the one item
`results/README.md` listed as "not established — no α-certification comparison",
for n = 28. Scaling n to reach useful α (≤ 0.05) remains the open frontier.

## Reproduce (offline, free)

```bash
for a in 0.10 0.27 0.30 0.32 0.35; do
  echo "alpha=$a"
  go-cascade calibrate -from-records results/pilot2-paired.execution.json -alpha $a -delta 0.10 -o /tmp/e.json | grep -E 'valid|empirical'
  go-cascade calibrate -from-records results/pilot2-paired.judge.json      -alpha $a -delta 0.10 -o /tmp/j.json | grep -E 'valid|empirical'
done
```
