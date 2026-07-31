# Completed n=64 pinned run — deployable α is reached, but not for free — 2026-07-30

Experiment 11 ([`go-specialist`](go-specialist-2026-07-25.md)) established that the
"~11% accuracy floor" was overwhelmingly spec-model test noise, and that removing
it (via the `-refs` oracle-soundness gate + `-pin-api`) **relocated the
deployable-α blocker back to sample size**. But that clean floor rested on **n=40**
from an externally-interrupted run — three of four long paired runs were SIGTERM'd
on the tail. The open step was a **completed n=64 pinned run** to (a) confirm the
low model-error rate at full n and (b) see whether it certifies a deployable α.

This is that run. The headline is a **tension**, not a clean win:

> **At n=64 the executable oracle certifies α=0.05 — the first deployable-α
> certificate in the study, overturning experiment 8's "the floor is model
> accuracy" for good. But at α=0.05 the cascade's cost advantage *inverts*: the
> certified thresholds collapse it to always-frontier-plus-wasted-cheap-calls.
> Deployable α and the 3.2× cost win are, at this fan-out, mutually exclusive.**

## Setup

Identical to experiment 11's 2:1:1 config (`config.go-specialist-211.json`): tier 0
Llama 4 Maverick ($0.24/$0.97, real us-west-2 rates), tiers 1–2 Claude
`sonnet-4-5` / `opus-4-5` at list price, oracle/judge `sonnet-4-6` (distinct from
every tier — invariant #3). `combined.jsonl` (n=64), `-refs examples/bench`
(all 64 references load across `refs/`, `hard/refs/`, `scale/refs/`), `-pin-api`,
`-alpha 0.05 -delta 0.10 -compare -baselines`.

**Ops — the resume fix is validated live.** The run was SIGTERM'd at 60/64 (the
same tail-kill that interrupted three prior long runs). PR #22's checkpoint/resume
worked exactly as designed: 59 records were already on disk, `-resume` skipped them
and ran only the final 5. This is the first live confirmation of the resume
machinery on a genuinely interrupted run — the ops blocker from experiment 11 is
closed. Live spend ≈ $2.2 in recorded tier calls plus spec/pin generation on
`sonnet-4-6` (well under the ~$6 estimate).

## Result 1 — genuine model-error rate is 0/52 at n=64

The `-refs -pin-api` gate reached a verdict on **100%** of problems (zero
inconclusive — the pin contract held on all 64). It excluded **12** as
oracle-unsound, in the three known spec-model noise classes:

- **6 broken-own-API** (missing imports / tests that don't compile against their
  own generated stub): `str_anagram`, `str_valid_ipv4`, `conc_parallel_map`,
  `conc_parallel_sum`, `conc_parallel_filter`, `conc_parallel_histogram`.
- **6 wrong-value / wrong-name / bad-assertion refutations** of the compiling
  reference: `scale_two_sum` (index order), `scale_fibonacci` (`Fibonacci(79)`
  wrong), `scale_hamming` (off-by-one), `scale_title_case`, `hard_conc_rate_limiter`
  (overdraft), `conc_safe_counter`.

On the **52 clean records the cascade is wrong on zero problems**. And the
striking part: **all 11 of the raw errors** (raw risk 11/64 = 0.1719, gate off)
are exactly the 11 escalation-exhausted noise problems — **genuine model errors =
0**. On this draw the measured "risk floor" was **100% spec-model test noise**,
even more extreme than experiment 11's ~93% at n=40.

*Honest reading:* 0/52 is a point estimate on one draw with a wide interval — the
claim is "genuine model error is rare and dominated by test noise on this
benchmark," **not** "the cascade is perfect." What experiment 11 argued at n=40,
n=64 confirms and sharpens: the floor was the oracle, not the model.

## Result 2 — α=0.05 certifies (the sample-size wall is cleared)

```
arm         valid  cert-α  emp-risk  real-risk  verdict
execution   true   0.050   0.0000    0.0000     certified risk holds
judge       false  0.050   0.0769    0.0000     could not certify
```

With 52 clean records ≥ the ~45 the paper's eq. 7 requires, and zero observed
errors, the execution arm certifies **α=0.05, δ=0.10, valid=true** (p=0.069 ≤ δ).
**This is the first deployable-α certificate in the study** and closes the thread
experiment 8 opened ("the floor is model accuracy, not sample size"): with the
oracle noise gated out, the floor was *neither* — it was test noise, and once
removed, n=52 is enough to certify α=0.05.

The **judge arm, on the same candidates, cannot certify α=0.05** (lowest
achievable empirical risk 0.077 > 0.05) — over-rejection inflating the risk it
certifies against, the §5.5 mechanism, paired and live once more.

## Result 3 — the cost win inverts at α=0.05 (the new finding)

`-baselines` compares the cascade against always-cheapest and always-frontier on
ground-truth cost and risk. At **α=0.05 the certified threshold vector is `[1, 1]`**:
the cheap tier's Wilson lower bound on 2 samples never reaches 1.0, so **no early
acceptance ever fires**. The cascade escalates to the frontier on every problem
*and* pays for the wasted cheap-tier calls first. Result, on the clean set (n=52):

| policy | risk | mean cost/query |
|--------|------|-----------------|
| always-cheapest | 0.115 | $0.00051 |
| **cascade (τ=[1,1])** | **0.000** | **$0.01313** |
| always-frontier | 0.000 | $0.00796 |

The cascade is **0.61× the cost of frontier — i.e. *more* expensive** — at
identical (zero) risk. Experiment 11's 3.2× cost win was measured at **α=0.19**,
where cheap-tier acceptance is certifiable. Sweeping α on the clean set shows the
two goals trade off directly:

| α | thresholds | cascade risk | cascade cost | vs frontier |
|---|-----------|-------------|-------------|-------------|
| 0.05 | [1, 1] | 0.000 | $0.01313 | **0.61× (pricier)** |
| 0.10 | [0.4, 1] | 0.019 | $0.00381 | 2.09× cheaper |
| 0.15 | [0.1, 1] | 0.038 | $0.00152 | 5.24× cheaper |
| 0.19 | [0.1, 1] | 0.038 | $0.00152 | 5.24× cheaper |

So **deployable α=0.05 and the cost win are not simultaneously achievable at this
fan-out.** You get one or the other. As α loosens, the threshold drops, the cheap
tier gets trusted, cost falls — and risk rises off zero. The lever has moved
again: it is no longer sample size (satisfied) but the **cheap tier's routing
signal at strict α** — a Wilson lower bound on 2 samples is structurally incapable
of clearing a threshold high enough to certify α=0.05, so the certificate has no
choice but to route everything to the frontier.

## What is established

1. **Deployable α=0.05 is reached at n=64** under the sound executable oracle
   (0 empirical, 0 realized risk) — the first in the study, and the judge arm
   cannot match it on the same candidates.
2. **The measured risk floor was 100% spec-model test noise on this draw** —
   genuine model errors 0/52, sharpening experiment 11's ~93%. Experiment 8's
   "the floor is model accuracy" is fully overturned.
3. **The `-pin-api` gate reaches a verdict on 100% of problems** at n=64 (zero
   inconclusive), excluding 12 across the three known noise classes.
4. **The cost win and deployable α are mutually exclusive at 2:1:1.** At α=0.05
   the certified thresholds force full escalation, making the cascade *pricier*
   than always-frontier. The 3.2× win holds only at α≥0.10, where risk leaves
   zero.
5. **The PR #22 resume fix works on a real interrupted run** — 59/64 checkpointed,
   `-resume` finished the tail, no data lost.

## Honest limitations

- **0/52 is one draw.** The interval is wide; do not read it as a true zero
  error rate. Cross-run risk numbers remain non-comparable (resampled tests).
- **The cost inversion is a property of the 2:1:1 fan-out and the 2-sample Wilson
  bound.** A higher cheap-tier fan-out would tighten the lower bound and *might*
  let a cheap acceptance certify at α=0.05 — untested. That is the natural next
  experiment: sweep the cheap-tier sample count and find the fan-out (if any)
  where α=0.05 certifies *with* a cost win.
- **`-baselines` risk uses all 64 records; the certificate uses the 52 clean.**
  The baseline table in Result 3 above is recomputed on the clean set for a
  like-for-like comparison; the raw tool output (all 64) shows cascade/frontier
  risk 0.1719 — that 0.1719 is the noise, not model error (see Result 1).
- **One benchmark, single judge model, small n.** Directions robust, magnitudes
  are point estimates.

## Reproduce

```bash
# Live (needs Bedrock; ~$2-6). If SIGTERM'd, re-run identical + -resume:
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.go-specialist-211.json \
  -bench examples/bench/combined.jsonl -refs examples/bench -pin-api \
  -alpha 0.05 -delta 0.10 -compare -baselines \
  -records results/go-specialist-211-pinned-n64.json

# Offline from the committed records (free):
go-cascade calibrate -from-records results/go-specialist-211-pinned-n64.execution.json \
  -alpha 0.05 -delta 0.10 -baselines -o /tmp/c.json    # valid=true, 12 unsound excluded, risk 0
go-cascade calibrate -from-records results/go-specialist-211-pinned-n64.judge.json \
  -alpha 0.05 -delta 0.10 -o /tmp/j.json                # valid=false (judge over-rejection)
```

Committed: `go-specialist-211-pinned-n64.execution.json`,
`…judge.json`, `…cert.json`. The 12 oracle-unsound flags and diagnostics are in
the execution records (`oracle_unsound` / `oracle_unsound_diag`).
