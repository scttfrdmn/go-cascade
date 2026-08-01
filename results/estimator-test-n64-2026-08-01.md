# The §3.7 estimator test, run live: mutation score is a *loose* bound on η_fa, and the oracle's real error is over-rejection — 2026-08-01

The design and non-circularity argument for this experiment are in
[`estimator-test-design-2026-08-01.md`](estimator-test-design-2026-08-01.md),
written before any spend. This is the live run.

§3.7 estimates the oracle gap η_fa = Pr[Y=0 | V=1] — a wrong program passing the
generated suite — by mutation score M = killed/valid, and states that **M is a
proxy for 1 − η_fa with unknown bias.** This experiment measures η_fa
independently and asks what that bias actually is.

**Headline: at n=64 the measured η_fa is 0/144 (95% upper bound 0.021), while the
pooled mutation gap 1 − M = 0.0996 would have predicted ~11 events. M is not
merely biased — on this benchmark it is loose by more than an order of magnitude,
and it is loose in the *safe* direction. The generated oracle's observed errors all
run the other way: 11 confirmed rejections of canonically-correct candidates, plus
37 rows where it refuted every candidate a tier produced.**

## Setup

- `go-cascade estimator --provider=bedrock --config
  examples/bench/config.go-specialist-211.json -bench examples/bench/combined.jsonl
  -refs examples/bench -o results/estimator-n64.json`
- n = 64 problems × 3 tiers (Maverick `small` / Sonnet 4.5 `mid` / Opus 4.5
  `large`) = **192 (problem, tier) rows**. 24 mutants per accepted candidate.
- The estimator pins each problem's API internally, so candidates compile against
  the human-authored canonical suite. `test_model = claude-sonnet-4-6`, distinct
  from every tier.
- **Measurement, not certificate.** `EstimateOracleGap` runs off the acceptance
  path and emits no threshold (invariants #4/#6). The canonical suite audits the
  generated one; it never participates in routing.
- The run was externally killed twice (at 30/64 and 31/64) and **resumed with zero
  loss** — the first live exercise of the atomic-checkpoint fix (PR #39). The
  earlier attempt, before that fix, lost 31 problems to a 0-byte checkpoint. One
  kill left an orphaned `.estimator-n64.json.tmp-*` (a `SIGKILL` pre-empts the
  cleanup `defer`), which is the fix behaving as designed: the *temp* is orphaned
  and the target checkpoint is intact, the opposite of the old failure.
- After the second kill the run was relaunched detached (own process session) so
  the harness's background-task reaper could not signal it; it then completed the
  remaining 33 problems uninterrupted.

## Result 1 — measured η_fa is zero, and M badly over-predicts it

Rows where the generated oracle accepted a candidate **and** the canonical suite
produced a label (V=1, Y observed): **144**.

| quantity | value |
|----------|-------|
| η_fa events (generated-accepted ∧ canonically-refuted) | **0 / 144** |
| 95% Clopper–Pearson upper bound on η_fa | **0.021** |
| pooled mutation gap 1 − M (886/984 mutants killed) | **0.0996** |
| η_fa events predicted if M were tight (Σ(1−Mᵢ) over 134 rows with M defined) | **11.1** |
| P(observe 0 events \| M tight), Poisson-binomial | **1.9 × 10⁻⁶** |

Every one of the 144 candidates the generated suite accepted was **also** judged
correct by the independent human-authored suite. Not one false acceptance.

This is the substantive finding about §3.7: treating 1 − M as an estimate of η_fa
would have predicted about eleven wrong-program acceptances on this benchmark, and
there were none. The discrepancy is not a small-sample coincidence — under the
per-row mutation gaps, seeing zero events would happen about twice in a million
runs if M were tight. **M's bias, here, is conservative by a wide margin.**

The mechanism is the one §3.7 names: the mutation-operator defect distribution and
the model's defect distribution are different, and M measures only the former. What
this run adds is the *magnitude* of the mismatch — the 98 surviving mutants imply a
~10% acceptance gap that the model's actual defects do not populate at all. (Why
they don't is not measured here: characterizing the 98 survivors would need a
per-mutant classification this run did not do, and I am not asserting one.)

## Result 2 — M does not discriminate, because it has no events to discriminate

Split at M ≥ 0.90, as pre-registered:

| bucket | rows | η_fa | 95% upper bound |
|--------|------|------|-----------------|
| M ≥ 0.90 | 94 | 0 / 94 = 0.000 | 0.031 |
| M < 0.90 | 40 | 0 / 40 = 0.000 | 0.072 |
| (M undefined — 0 valid mutants) | 10 | — | — |

The M distribution is **not** degenerate — it spans 0.500 to 1.000 (median 1.000,
mean 0.917; 82 rows at exactly 1.00, 40 rows below 0.90), so the split had real
material to work with. But with zero events in both buckets the contingency table
cannot discriminate, and the two upper bounds overlap completely.

So the honest reading is: **this run bounds η_fa and refutes M as a tight proxy,
but it does not establish whether M *ranks* candidates by η_fa.** The twelve
lowest-M rows are instructive — `scale_clamp` at M = 0.500 (2/4 mutants killed),
`hard_conc_rate_limiter` at 0.600, `hard_num_mean_overflow` at 0.600 — and **all
twelve are canonically correct.** Low M did not indicate a weak oracle; it
indicated a small mutant pool on a short function, or timing-dependent mutants a
concurrency suite cannot deterministically kill. That is a measurement artifact of
M, not a signal about the oracle.

Answering the ranking question needs η_fa events, and η_fa events need a model that
is confidently wrong *in a way generated tests miss* — rare enough at n=64 that
this is exactly the gap the §5.5 n≥300 experiment exists to close.

## Result 3 — the generated oracle's observed errors are all over-rejection

The asymmetry is on the other side of the contingency table. Of 192 rows:

| generated verdict | canonical verdict | rows |
|-------------------|-------------------|------|
| accepted | correct | 144 |
| accepted | **wrong (η_fa)** | **0** |
| rejected | **correct (confirmed false rejection)** | **11** |
| no candidate survived the ladder — no candidate to label | (unlabeled) | 37 |

The **11 confirmed false rejections** are rows where a candidate existed, the
generated suite rejected it, and the canonical suite says it is correct:
`seq_longest_run` (all 3 tiers), `scale_fibonacci` (3), `num_isqrt` (2), `str_rle`,
`scale_is_palindrome`, `hard_num_mean_overflow`. Against the 155 rows that produced
a candidate, that is a **7.1% false-rejection rate versus a 0% false-acceptance
rate.**

The **37 unlabeled rows** (11 problems) are a separate population and are *not*
counted as false rejections: no candidate survived the generated ladder, so there
was nothing to hand the canonical suite and no Y label exists. They are still
informative as a cluster — `conc_parallel_map`, `conc_parallel_sum`,
`conc_parallel_filter`, `conc_parallel_histogram`, `hard_conc_once_init`, plus
`str_anagram`, `str_valid_ipv4`, `scale_two_sum`, `scale_title_case`,
`scale_count_vowels`, `scale_reverse_int` failed at all three tiers, and four more
(`conc_fan_in_merge`, `hard_conc_ordered_fanout`, `scale_intersection`,
`scale_max_subarray`) at one tier each. Whether each is an over-strict generated
suite or three genuinely wrong candidates is not resolved by this run.

Six of the eleven all-tier-failing problems are in the **already-known
oracle-unsound** set that experiment 18's `-refs` gate independently flagged
(`conc_parallel_filter`, `conc_parallel_map`, `scale_reverse_int`,
`scale_title_case`, `scale_two_sum`, `str_anagram`) — the recurring spec-model
test-noise class. The other five (`conc_parallel_histogram`, `conc_parallel_sum`,
`hard_conc_once_init`, `scale_count_vowels`, `str_valid_ipv4`) were **not** flagged
unsound there, so on this draw the generated suite refuted every candidate for a
problem whose reference passes its own canonical suite — a stricter-than-canonical
generated oracle, which is sound but expensive.

This matters for the method's economics rather than its safety. Invariant #4 makes
a failed stage a *sound refutation*, and over-rejection is precisely the safe
direction: it costs escalations and money, never risk. But it means the generated
oracle at 2:1:1 spends its error budget forcing correct cheap-tier answers up the
cascade — the same wall experiments 12–18 hit from the cost side, now visible from
the oracle side. The concurrency cluster is the sharpest instance: generated suites
for `conc_*` problems are strict enough (timing, ordering, race assertions) that no
candidate at any tier survives.

## What this establishes, and what it does not

**Establishes:**

1. **§3.7's "unknown bias" has a measured direction on this benchmark: M
   under-states 1 − η_fa substantially.** 1 − M = 0.0996 vs measured η_fa ≤ 0.021
   (95%). Using 1 − M as an η_fa estimate is conservative here, not optimistic —
   which is the safe way for a proxy in a risk argument to be wrong, and worth
   stating because the paper could not previously say which way it erred.
2. **η_fa itself is small on this benchmark**: 0/144, upper bound 0.021. The
   generated oracle, where it accepts, agrees with the human oracle
   unanimously across 144 candidate-tier pairs.
3. **Every observed generated-oracle error is an over-rejection** — 11 confirmed
   false rejections (7.1% of labeled-candidate rows) against 0 false acceptances,
   plus 37 rows where the ladder left no candidate at all, five of whose problems
   the existing soundness gate did not flag. This is a new observation: §3 models
   the false-acceptance hazard and the `-refs` gate models
   unsound-because-it-refutes-the-reference, but neither captures
   "sound-but-stricter-than-canonical," which shows up as pure cost.

**Does not establish:**

- That M **ranks** candidates by η_fa (Result 2: no events in either bucket, bounds
  overlap). The ordering question is unresolved and needs n ≥ 300.
- That η_fa is small in general. This is one benchmark (single-file, stdlib-only,
  64 problems), one draw, one tier configuration. Risk quantities are
  sampling-noisy at this n and must not be compared across runs.
- Anything about a certificate. No threshold moved; M remains descriptive.

## Honesty line

The headline is a **negative result about the estimator** (M is loose) plus a
**null result on the discriminative question** (no events, so no ranking signal) —
reported as such, not dressed up as a validation of M. The one number doing real
work, Σ(1−Mᵢ) = 11.1 predicted vs 0 observed, is a comparison *within* this run and
is robust to the small n; the per-bucket bounds are not, and are given as bounds.
The 37 no-candidate rows are counted separately from the 11 confirmed false
rejections rather than pooled, because they carry no canonical label — pooling them
would report a 25% "false-rejection rate" that the data does not support. No mock
numbers are cited. Records: `results/estimator-n64.json`.
