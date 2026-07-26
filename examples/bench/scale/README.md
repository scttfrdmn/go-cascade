# Scale benchmark tier

24 additional problems whose purpose is **sample size**, not difficulty. The
certification comparison (`results/certification-comparison-2026-07-25.md`)
showed the executable oracle certifies a lower risk bound than the judge, but at
n = 28 the best certifiable α was ~0.27 — far above the ≤ 0.05–0.10 range anyone
would deploy. That ceiling is set by n, not by the oracle: certifying α = 0.05 at
δ = 0.10 needs **n ≥ 45** even at zero observed errors (paper eq. 7).

This tier raises the combined benchmark to **64 problems** (`examples/bench/`
28 + `hard/` 12 + `scale/` 24), clearing that floor. The problems are
deliberately **tractable and stdlib-only** — sum-of-digits, FizzBuzz, Kadane,
two-sum, Caesar cipher, and similar — so a strong model solves them cleanly. That
matters twice over: it lifts n, and it keeps the *empirical risk floor* low, since
the certifiable α can never drop below the observed error rate no matter how large
n grows.

Each problem ships an execution-validated reference (`refs/<id>/`), same contract
as the other tiers. `validate.sh` compiles and tests all 24; all pass.

## Running the scaled certification comparison

```bash
# Combined benchmark (all three tiers, n=64), paired oracle comparison:
AWS_PROFILE=<profile> go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.example.json \
  -bench examples/bench/combined.jsonl -alpha 0.05 -delta 0.10 -compare -records scaled.json

# Then find the lowest certifiable alpha per arm by replaying offline (free):
for a in 0.05 0.08 0.10 0.12 0.15 0.20; do
  go-cascade calibrate -from-records scaled.execution.json -alpha $a -delta 0.10 -o /tmp/e.json | grep -E 'valid|empirical'
  go-cascade calibrate -from-records scaled.judge.json      -alpha $a -delta 0.10 -o /tmp/j.json | grep -E 'valid|empirical'
done
```

The open question this tier is built to answer: **with n = 64, how low a
certified α does each oracle reach, and does the executable oracle's advantage
hold at a deployable bound?** The honest limit remains the empirical risk floor —
if the models' true error rate on this benchmark is ~10%, α = 0.05 is unreachable
regardless of n, and the result will say so.
