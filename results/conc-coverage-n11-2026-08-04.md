# Concurrency coverage: the race rung fires, and a generated test can be *non-terminating* rather than wrong — 2026-08-04

Issue #50. Experiment 25.

MultiPL-E Go has **0** concurrency problems, so `verify.Static.UsesConcurrency` was
false for all 488 records of experiment 21 and the `-race` rung — the rung that
caught the study's only confirmed judge over-acceptance — was **skipped every single
time** in the only large-n run. A skipped stage scores `OK`, so nothing failed and
nothing printed. This run exercises that rung on the 11 concurrency problems of the
hand-written benchmark.

> **n = 11 is small by construction. This is coverage, not a certificate, and the
> arithmetic is worth stating exactly: at δ=0.10, `MinCalibrationSize` requires
> α ≥ 0.19 for n=11 even at zero empirical errors. The run was launched at α=0.05, so
> `valid=false` on both arms is a statement about the sample size, not about either
> oracle. Do not read the certification comparison off this run — that is
> experiment 21's job (α: execution 0.084 vs judge 0.226 at n=409).**

**Headline: the run's finding is not about concurrency, it is about the oracle's
clock. One of 11 records was excluded as `OracleUnsound` because the *reference*
was refuted at `VA:accept` — and the refutation was a timeout, reproduced
deterministically as a generated test that needs ~20 s to finish inside a 30 s
budget already spent on ten other tests. Without the `TimedOut` flag shipped
three hours earlier (#63), this run would have reported η_fa = 3/26 = 0.115 —
11× experiment 21's 11/1096 — from a single non-terminating test.**

## Setup

```bash
AWS_PROFILE=aws go-cascade calibrate -provider=bedrock -compare \
  -config examples/bench/config.go-specialist-211.json \
  -bench examples/bench/concurrency.jsonl \
  -refs examples/bench -pin-api \
  -alpha 0.05 -delta 0.10 -baselines -resume \
  -records results/conc-n11.records.json -o results/conc-n11.cert.json

# every figure below, from the committed records, free
python3 results/analyze_s55.py results/conc-n11.records
python3 results/classify_disagreements.py results/conc-n11.records.judge.json --dump /tmp/disag
```

- **No `-deadline`.** It sets `Options.SkipRace`, which would skip the rung this run
  exists to exercise. That is the one flag that silently defeats the experiment.
- `examples/bench/concurrency.jsonl`, 11 problems: 8 from `problems.jsonl`, 3 from
  `hard/`, 0 from `scale/`. **Exactly 11 of the 64 references trip the predicate**, so
  this is the complete concurrency set, not a subset — asserted by
  `TestConcurrencyBenchActuallyReachesTheRaceRung`, in both directions (every id in
  the file trips it; no concurrency reference anywhere is absent from the file).
- Same config as experiments 12 and 21 (`config.go-specialist-211.json`, 2:1:1
  Maverick/Sonnet 4.5/Opus 4.5, `test_model=sonnet-4-6`) so cross-run comparison holds.
- `RaceCount: 3`, `TestTimeout: 30s` — both `config.Default()`. This is the study's
  most verifier-heavy configuration per problem, which is why #63 was done first.
- **Bill $0.34 of tier+judge spend plus ~$0.45 of unrecorded spec cost ≈ $0.8**,
  against $1–2 scoped. Wall clock 13 min, 11/11, no kill. The spec term is 58% of
  this run's bill and appears in no record — see `results/README.md`
  §"A correction to every cost figure on this page".

## Result 1 — the rung fires

`V5:race` executes, `skipped=false`, on all 11. Measured on a reference through the
real ladder: 2.141 s at `-count=3` against 605 ms for the plain test rung, ~3.5×.
Experiment 21 ran this rung zero times across 488 problems; it now has coverage.

## Result 2 — the finding: a generated test that cannot terminate

`conc_safe_counter` was flagged `OracleUnsound` and excluded. Read back from the
record, the reference passed **10** hidden tests and then the output stops mid-test,
inside `TestHInt64Overflow`. All three of its tier observations carry `timed_out`.

The reference is not slow. Against its own **canonical** suite it passes in 673 ms at
a 30 s timeout and 435 ms at 120 s — so this was never machine load, which is what
the WARNING that `calibrate` printed invites you to assume:

> `timeouts (shared candidate stream): WARNING — 1/11 records had a verifier stage
> killed by the clock … Re-run on an idle host, or raise test_timeout, before quoting
> it.`

Reproduced instead, deterministically and for $0. `ConcurrentCount` increments by
**one** per operation, so "overflow an int64 counter" taken literally is ~2^63
sequential atomic adds. Measured rate: **215 M increments/s**.

| generated test shape | wall time |
|---|---|
| `ConcurrentCount(2, math.MaxInt32)` | **20.0 s** — passes, alone |
| `ConcurrentCount(4, math.MaxInt32)` | **>40 s** — over budget on its own |
| reaching `MaxInt64` by +1 | ~1361 years |

The assertion is *correct*. The test is *sound*. It simply cannot finish, and it ran
eleventh in a 30 s budget. So the exclusion is right — a record whose truth column
came from a clock is not evidence — but the **reason** printed in
`oracle_unsound_diag` ("reference refuted at VA:accept") names the wrong cause, and
the fix is not an idler host or a bigger timeout. It is that a spec model asked for a
boundary test on a unit-increment counter writes a program with no reachable end.

**This is a new hazard class for the study.** Experiment 19 established that the
generated oracle's errors are all over-*rejections* — sound but stricter than
canonical, costing escalations rather than risk. This is a third kind: an oracle test
that is neither wrong nor strict but **non-terminating**, and whose refutation is
indistinguishable from a slow machine without `TimedOut`. It is specific to
concurrency problems only in that concurrency problems invite "hammer it with N
operations" tests; nothing about the mechanism is concurrent.

### What it would have cost to not have the flag

| | over-accept (η_fa) | denominator |
|---|---|---|
| this run, timeout-flagged record excluded | **0** | 23 |
| this run, as it would have read pre-#63 | **3** | 26 |
| experiment 21, n=409 | 11 | 1096 |

3/26 = **0.115**, against experiment 21's 0.0100. A single non-terminating test on a
single problem, tripling three tiers at once because the truth column is shared,
would have produced the study's largest η_fa figure by an order of magnitude — on
the one defect class the paper's mechanism argument is *about*. #63 shipped hours
before this run purely as instrumentation; it changed a headline on first live use.

Two properties of the #63 design earned their keep here, both deliberate:

- **The count is over records, not observations.** `conc_safe_counter` timed out on
  all three tiers; the tally says `1/11`, one suspect problem. Reported as three
  events it would have read as a pattern.
- **The record is kept, not dropped** (invariant #8). It is `OracleUnsound` that
  excludes it, on the oracle's own soundness gate. Filtering on `timed_out` would
  select the calibration sample on an outcome.

## Result 3 — η_fa is 0/23 here; the judge's errors are all over-rejections, and they are *legible*

`classify_disagreements.py` (issue #49) had never had live data. This is its first
run, and the retention worked: **6/6 disagreements carry source**.

| direction | count | denominator |
|---|---|---|
| agree | 17 | 23 |
| **over-accept (η_fa)** | **0** | 23 |
| over-reject (β) | 6 | 23 |

Tier gradient of the over-rejections: **small 4, mid 2, large 0** — the §3.1
direction, on the safe side.

Reading the six programs is the point, and they share one signature: **every one is
correct concurrent code that reads as suspicious.**

- `conc_parallel_map` (small) — disjoint `out[start+j]` slots per goroutine,
  provably race-free, but the judge sees unsynchronised writes to a shared slice.
- `conc_parallel_filter` (small) — stride partitioning `j%workers == workerID` into a
  shared `decisions` slice. Same shape: disjoint by construction, shared by appearance.
- `conc_parallel_histogram` (small, mid) — per-goroutine local maps merged under a
  mutex; 6 concurrency tokens including `make(chan`.
- `conc_first_success` (small, mid) — `context.WithCancel` + mutex-guarded
  first-writer-wins, correct, and dense enough to look racy.

This is the **mirror image** of the paper's mechanism claim. §3.5 argues the judge's
blind spot is reading-*invisible* defects (it passes racy code that looks fine). Here
the same blind spot runs the other way: it *fails* race-free code that looks racy.
Both follow from the judge having no execution — it is reasoning about the text, and
disjoint-index partitioning is exactly the property text does not show. Over-rejection
is the safe direction (escalations, not risk), which is why it is the cheaper half of
the same phenomenon and why the judge loses the certification race in experiment 21
mostly on over-rejection.

**What this does not establish.** η_fa = 0/23 at n=11 is not evidence that the judge
does not over-accept on concurrency — the pilot's single confirmed over-acceptance was
a *scar-free* race, and none of the model-authored candidates here contained one. The
class that produced it remains unsampled, which is what the scar-free race operator
(issue #51) is for. That gap is now sharper, not closed: this run confirms the judge
mis-reads concurrency in the *legible* direction, on 6 programs that can be read.

## Result 4 — two more generated-oracle defects, both compile-time

The other two exclusions are unrelated to the clock and are ordinary spec-model bugs,
caught by `-refs` exactly as designed (invariant #4):

- `hard_conc_once_init` — `visible_test.go:119:9: undefined: sync`. The generated
  test uses `sync` without importing it.
- `hard_conc_ordered_fanout` — `hidden_test.go:91:6: assignment copies lock value to
  _: sync/atomic.Int64 contains sync/atomic.noCopy`. Caught by `go vet`, which is in
  the ladder for precisely this reason.

So **3 of 11 records excluded, all three on oracle defects, none on model output.**

The right comparison is available directly rather than against a pooled rate:
experiment 12 ran this same config over all 64 problems, so its 12 unsound records
can be split by class.

| set | oracle-unsound | rate |
|---|---|---|
| the 11 concurrency problems (experiment 12) | 6 | **0.55** |
| the other 53 (experiment 12) | 6 | 0.11 |
| the 11 concurrency problems (this run) | 3 | 0.27 |

**Concurrency problems produce an unsound generated oracle ~5× as often** — 6/11
against 6/53 on identical config, spec model and pinning. That is a property of the
benchmark class, not of this draw, and it is a coverage cost nobody was accounting
for: the class the paper most needs (races) is the class whose oracle is least often
trustworthy, so usable n shrinks fastest exactly where it is scarcest.

**The two draws agree on exactly one id.** `conc_safe_counter` is unsound in both;
`hard_conc_once_init` and `hard_conc_ordered_fanout` are new here, while
`conc_parallel_map`, `_sum`, `_filter`, `_histogram` and `hard_conc_rate_limiter` all
recovered. That is 1 of 8 ids ever flagged appearing in both draws — a direct
reproduction of experiment 19's finding that **rejection-side rates are not stable at
this n**, now on a different subcommand and a different oracle. Treat both rates as
evidence of a large effect, not as point estimates, and note that the *one* stable
flag is the non-terminating test — a deterministic defect, unlike the compile errors
a redraw fixes.

## Reproduce

```bash
# free, from the committed records
python3 results/analyze_s55.py results/conc-n11.records
python3 results/classify_disagreements.py results/conc-n11.records.judge.json --dump /tmp/disag

# the coverage assertion
go test ./cmd/go-cascade/ -run TestConcurrencyBenchActuallyReachesTheRaceRung -v
```

Records: `results/conc-n11.records.execution.json`,
`results/conc-n11.records.judge.json`. No certificate file: neither arm certifies at
n=11, and `-o` writes nothing when nothing is valid.
