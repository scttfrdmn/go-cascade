# The §3.7 estimator test, run live: mutation score is a *loose* bound on η_fa, and the oracle's real error is over-rejection — 2026-08-01

The design and non-circularity argument for this experiment are in
[`estimator-test-design-2026-08-01.md`](estimator-test-design-2026-08-01.md),
written before any spend. This is the live run.

> **This write-up was corrected on 2026-08-01, after a bug was found in the first
> run's oracle.** `canonicalVerdict` handed the canonical suite to the ladder as the
> *visible* partition, so `-run ^TestV` applied and **222 of the 370 canonical tests
> never executed** — by construction the adversarial ones. Every number below now
> comes from the re-run against the full suite (PR #42, records
> `results/estimator-n64-full-oracle.json`); the weakened-oracle records are kept as
> `results/estimator-n64.json` so the two can be compared. The section
> "[What the weakened oracle changed](#what-the-weakened-oracle-changed)" reports
> the diff, because it is the most informative part of this experiment.

§3.7 estimates the oracle gap η_fa = Pr[Y=0 | V=1] — a wrong program passing the
generated suite — by mutation score M = killed/valid, and states that **M is a
proxy for 1 − η_fa with unknown bias.** This experiment measures η_fa
independently and asks what that bias actually is.

**Headline: at n=64 the measured η_fa is 0/145 (95% upper bound 0.020), while the
pooled mutation gap 1 − M = 0.1014 would have predicted ~12 events. M is not
merely biased — on this benchmark it is loose by more than an order of magnitude,
and it is loose in the *safe* direction. The generated oracle's observed errors all
run the other way: 4 confirmed rejections of canonically-correct candidates, plus
40 rows where it refuted every candidate a tier produced.**

## Setup

- `go-cascade estimator --provider=bedrock --config
  examples/bench/config.go-specialist-211.json -bench examples/bench/combined.jsonl
  -refs examples/bench -o results/estimator-n64-full-oracle.json`
- n = 64 problems × 3 tiers (Maverick `small` / Sonnet 4.5 `mid` / Opus 4.5
  `large`) = **192 (problem, tier) rows**. 24 mutants per accepted candidate.
- The estimator pins each problem's API internally, so candidates compile against
  the human-authored canonical suite. `test_model = claude-sonnet-4-6`, distinct
  from every tier.
- **Measurement, not certificate.** `EstimateOracleGap` runs off the acceptance
  path and emits no threshold (invariants #4/#6). The canonical suite audits the
  generated one; it never participates in routing.
- **845 canonical tests executed across the 145 labeled rows** (2–14 per row).
  This total is now recorded per row in `EstimatorObs.CanonicalTests`, which is
  what makes oracle strength auditable from the records rather than assumed — the
  field exists *because* its absence is what let the first run's bug go unnoticed.
- All 64 references were verified to pass their own **full** canonical suites before
  the run (0 failures), so the stronger oracle is itself sound. An oracle that
  refutes a correct reference would make its labels noise, not evidence
  (invariant #4).
- Every figure here is computed by `results/analyze_estimator.py`, which reproduces
  the first run's published numbers exactly from its records. So where the two runs
  differ, the oracle changed — not the arithmetic.
- The re-run completed all 64 problems uninterrupted in ~85 minutes.

## Result 1 — measured η_fa is zero, and M badly over-predicts it

Rows where the generated oracle accepted a candidate **and** the canonical suite
produced a label (V=1, Y observed): **145**.

| quantity | value |
|----------|-------|
| η_fa events (generated-accepted ∧ canonically-refuted) | **0 / 145** |
| 95% Clopper–Pearson upper bound on η_fa | **0.020** |
| pooled mutation gap 1 − M (975/1085 mutants killed) | **0.1014** |
| η_fa events predicted if M were tight (Σ(1−Mᵢ) over 138 rows with M defined) | **11.7** |
| P(observe 0 events \| M tight), Poisson-binomial | **1.0 × 10⁻⁶** |

Every one of the 145 candidates the generated suite accepted was **also** judged
correct by the independent human-authored suite, running in full. Not one false
acceptance.

This is the substantive finding about §3.7: treating 1 − M as an estimate of η_fa
would have predicted about twelve wrong-program acceptances on this benchmark, and
there were none. The discrepancy is not a small-sample coincidence — under the
per-row mutation gaps, seeing zero events would happen about once in a million
runs if M were tight. **M's bias, here, is conservative by a wide margin.**

One asymmetry is worth stating precisely, because it is what makes this comparison
survive the oracle bug: **M was never measured through the broken path.**
`verify.Mutate` runs `go test ./...` with no `-run` filter, so the "predicted ~12"
side of the comparison was computed against the whole generated suite in both runs.
The bug weakened only the canonical Y oracle — the "0 observed" side. So the first
run's headline was measured too weakly in exactly the direction that would have
manufactured this result, which is why it had to be re-run rather than argued about.

The mechanism is the one §3.7 names: the mutation-operator defect distribution and
the model's defect distribution are different, and M measures only the former. What
this run adds is the *magnitude* of the mismatch — the 110 surviving mutants imply a
~10% acceptance gap that the model's actual defects do not populate at all. (Why
they don't is not measured here: characterizing the survivors would need a
per-mutant classification this run did not do, and I am not asserting one.)

## Result 2 — M does not discriminate, because it has no events to discriminate

Split at M ≥ 0.90, as pre-registered:

| bucket | rows | η_fa | 95% upper bound |
|--------|------|------|-----------------|
| M ≥ 0.90 | 96 | 0 / 96 = 0.000 | 0.031 |
| M < 0.90 | 42 | 0 / 42 = 0.000 | 0.069 |
| (M undefined — 0 valid mutants) | 7 | — | — |

The M distribution is **not** degenerate — it spans 0.500 to 1.000 (median 1.000,
mean 0.915; 79 rows at exactly 1.00, 42 rows below 0.90), so the split had real
material to work with. But with zero events in both buckets the contingency table
cannot discriminate, and the two upper bounds overlap completely.

So the honest reading is: **this run bounds η_fa and refutes M as a tight proxy,
but it does not establish whether M *ranks* candidates by η_fa.** The twelve
lowest-M rows are instructive — `scale_clamp` at M = 0.500 (2/4 mutants killed, all
three tiers), `hard_conc_rate_limiter` at 0.600, `scale_intersection` and
`scale_reverse_int` at 0.625 — and **all twelve are canonically correct.** Low M did
not indicate a weak oracle; it indicated a small mutant pool on a short function, or
timing-dependent mutants a concurrency suite cannot deterministically kill. That is
a measurement artifact of M, not a signal about the oracle.

Answering the ranking question needs η_fa events, and η_fa events need a model that
is confidently wrong *in a way generated tests miss* — rare enough at n=64 that
this is exactly the gap the §5.5 n≥300 experiment exists to close.

## Result 3 — the generated oracle's observed errors are all over-rejection

The asymmetry is on the other side of the contingency table. Of 192 rows:

| generated verdict | canonical verdict | rows |
|-------------------|-------------------|------|
| accepted | correct | 145 |
| accepted | **wrong (η_fa)** | **0** |
| rejected | **correct (confirmed false rejection)** | **4** |
| rejected | wrong (both oracles agree it is wrong) | 3 |
| no candidate survived the ladder — no candidate to label | (unlabeled) | 40 |

The **4 confirmed false rejections** are rows where a candidate existed, the
generated suite rejected it, and the full canonical suite says it is correct: all
four are concurrency problems — `hard_conc_once_init` (all 3 tiers) and
`hard_conc_ordered_fanout` (1 tier). Against the 152 rows that produced a candidate,
that is a **2.6% false-rejection rate versus a 0% false-acceptance rate.**

The **3 agreed-wrong rows** are new information the weakened oracle could not
produce, and they are *not* η_fa events — the generated suite rejected them too. See
the next section: all three were refuted by `TestH*` functions.

The **40 unlabeled rows** (18 problems) are a separate population and are *not*
counted as false rejections: no candidate survived the generated ladder, so there
was nothing to hand the canonical suite and no Y label exists. They are still
informative as a cluster — `conc_parallel_map`, `conc_parallel_sum`,
`conc_parallel_filter`, `conc_parallel_histogram`, `hard_num_pow_mod`,
`scale_flatten`, `scale_hamming`, `scale_title_case`, `scale_two_sum`,
`str_anagram`, `str_valid_ipv4` failed at all three tiers (33 rows), and seven more
problems at exactly one tier each (7 rows). Whether each is an over-strict generated
suite or genuinely wrong candidates is not resolved by this run.

Five of the eleven all-tier-failing problems are in the **already-known
oracle-unsound** set that experiment 18's `-refs` gate independently flagged
(`conc_parallel_filter`, `conc_parallel_map`, `scale_title_case`, `scale_two_sum`,
`str_anagram`) — the recurring spec-model test-noise class. The other six were
**not** flagged unsound there, so on this draw the generated suite
refuted every candidate for a problem whose reference passes its own canonical suite
— a stricter-than-canonical generated oracle, which is sound but expensive.

This matters for the method's economics rather than its safety. Invariant #4 makes
a failed stage a *sound refutation*, and over-rejection is precisely the safe
direction: it costs escalations and money, never risk. But it means the generated
oracle at 2:1:1 spends its error budget forcing correct cheap-tier answers up the
cascade — the same wall experiments 12–18 hit from the cost side, now visible from
the oracle side. The concurrency cluster is the sharpest instance: generated suites
for `conc_*` problems are strict enough (timing, ordering, race assertions) that no
candidate at any tier survives, and the only confirmed false rejections in the run
are also concurrency problems.

## What the weakened oracle changed

The first run's oracle executed 148 of 370 canonical tests (**40.0%**), skipping
every `TestH*` function. Comparing the two runs row by row is the clearest evidence
of what a weakened oracle costs, and it is more interesting than the headline:

| quantity | 40% oracle | full oracle |
|----------|-----------|-------------|
| canonical tests executed (recorded) | not recorded | **845** |
| labeled V=1 rows | 144 | 145 |
| **η_fa events** | **0** | **0** |
| 95% bound on η_fa | 0.021 | 0.020 |
| pooled 1 − M | 0.0996 | 0.1014 |
| predicted events if M tight | 11.1 | 11.7 |
| confirmed false rejections | **11** | **4** |
| agreed-wrong rows | 0 | **3** |
| no-candidate rows | 37 | 40 |

**The headline survived; the second-order findings did not.** Three things follow,
and only the first was predictable:

1. **The oracle now refutes things it could not see.** All **3** canonical
   refutations in the re-run come from `TestH*` functions — `TestHMaxUint64`,
   `TestHOutOfRange`, `TestHInputNotModified` — and the weakened run produced
   **zero** canonical refutations of any kind. The defects are real:
   `IntSqrt(MaxUint64) = 4294967296, want 4294967295` and
   `Fibonacci(94) = 1293530146158671551, want 0` are an off-by-one at the integer
   bound and an int64 overflow. The 40% oracle called both **correct**. This is the
   fix working, demonstrated on live records rather than argued.

2. **The false-rejection rate was overstated, and its problem set was wrong.**
   11 → 4 (7.1% → 2.6%), and the *identity* of the rows changed completely: of the
   old 11, **eight** were re-drawn as candidates the full oracle accepts, **two**
   (`num_isqrt` mid, `scale_fibonacci` small) are now **agreed-wrong** — the full
   oracle refutes what the 40% oracle called correct, so the generated suite was
   right to reject them and they were never false rejections at all — and one
   produced no candidate. **Not one of the old 11 remains a false rejection**, and
   the four that are one now (`hard_conc_once_init` ×3,
   `hard_conc_ordered_fanout` ×1) are a disjoint, entirely concurrency-flavoured
   set. The "sound-but-stricter-than-canonical" hazard class
   is still real and still the only observed error direction, but at n=64 its
   magnitude is not something this benchmark pins down.

3. **Row-level agreement between the two runs is 159/192 (83%)** — 33 rows changed
   class, in both directions (11 `acc/ok → no-cand`, 6 `no-cand → acc/ok`, 3
   `no-cand → rej/ok`). Most of that churn is *sampling*, not the oracle: each run
   draws fresh candidates at temperature > 0, and a class change that does not
   involve a canonical verdict flip cannot be attributed to the fix. It is the
   single strongest caution in this write-up — **risk and rejection quantities at
   n=64 are not stable across draws**, so no number here should be compared against
   a number from any other run, including its own predecessor. That is the §5.5
   n≥300 argument in one line.

## What this establishes, and what it does not

**Establishes:**

1. **§3.7's "unknown bias" has a measured direction on this benchmark: M
   under-states 1 − η_fa substantially.** 1 − M = 0.1014 vs measured η_fa ≤ 0.020
   (95%). Using 1 − M as an η_fa estimate is conservative here, not optimistic —
   which is the safe way for a proxy in a risk argument to be wrong, and worth
   stating because the paper could not previously say which way it erred. The
   comparison is non-circular twice over: M is measured against the *generated*
   suite while Y comes from the *human-authored* one, and M was never affected by
   the oracle bug, so the predicted side of the comparison is unchanged.
2. **η_fa itself is small on this benchmark**: 0/145, upper bound 0.020. The
   generated oracle, where it accepts, agrees with the full human oracle unanimously
   across 145 candidate-tier pairs.
3. **Every observed generated-oracle error is an over-rejection** — 4 confirmed
   false rejections (2.6% of labeled-candidate rows) against 0 false acceptances,
   plus 40 rows where the ladder left no candidate at all. This is a new
   observation: §3 models the false-acceptance hazard and the `-refs` gate models
   unsound-because-it-refutes-the-reference, but neither captures
   "sound-but-stricter-than-canonical," which shows up as pure cost.
4. **An oracle's strength has to be measured, not assumed.** A suite can execute
   40% of itself and report `ok`, and nothing in the risk argument notices, because
   every stage still returns a sound verdict on the tests it *did* run. The
   `canonical_tests` field and the zero-tests-ran guards (PR #42) exist so this
   class of defect fails loudly instead of silently producing a publishable number.

**Does not establish:**

- That M **ranks** candidates by η_fa (Result 2: no events in either bucket, bounds
  overlap). The ordering question is unresolved and needs n ≥ 300.
- That η_fa is small in general. This is one benchmark (single-file, stdlib-only,
  64 problems), one draw, one tier configuration.
- **Any stable value for the false-rejection rate.** Two draws of the same
  experiment gave 7.1% and 2.6% on disjoint problem sets. The *direction* of the
  asymmetry replicated; the magnitude did not.
- Anything about a certificate. No threshold moved; M remains descriptive.

## Honesty line

The headline is a **negative result about the estimator** (M is loose) plus a
**null result on the discriminative question** (no events, so no ranking signal) —
reported as such, not dressed up as a validation of M. The one number doing real
work, Σ(1−Mᵢ) = 11.7 predicted vs 0 observed, is a comparison *within* this run and
is robust to the small n; the per-bucket bounds are not, and are given as bounds.
The 40 no-candidate rows are counted separately from the 4 confirmed false
rejections rather than pooled, because they carry no canonical label — pooling them
would report a 23% "false-rejection rate" that the data does not support.

The first version of this write-up reported the same headline from an oracle running
40% of its tests. The headline held up; two of its three secondary findings did not,
and the false-rejection paragraph was wrong in both magnitude and membership. That
correction is recorded above rather than quietly overwritten, because the failure
mode — a weakened oracle producing a *plausible* number — is the one this method is
supposed to be most careful about. No mock numbers are cited. Records:
`results/estimator-n64-full-oracle.json` (full oracle) and
`results/estimator-n64.json` (the superseded 40% run, retained for comparison).
