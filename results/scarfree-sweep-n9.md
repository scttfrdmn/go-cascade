# The scar-free race sweep ran, and BOTH arms are null: 0/9 and 0/27 (experiment 30)

**Date:** 2026-08-05 · **Bill: PENDING RECONCILIATION** (priced ex ante at ~$1.20,
ceiling ~$1.94) · Two arms, same 11 concurrency problems, same session, ~32 min

> **Reconciling this bill needs a third filter, not the two documented ones.** Cost
> Explorer lags ~1 day, so 2026-08-05 reads $0 while the run executes. The known traps
> are that Claude bills under `Amazon Bedrock Service` (not `Amazon Bedrock`, which holds
> only the open-weight models) and that the service line contains Claude Code's own
> usage. The **third**, visible on 2026-08-04: Claude Code's traffic is
> `us.anthropic.claude-{opus,sonnet}-5` with a **`-mantle-`** infix, but the study's spec
> model bills as `Claude4.6Sonnet` — and that *same* usage-type family carries **$18.97
> of `cache-read-input-token-count`** on a day the study spent a few dollars.
> go-cascade sends no cached prompts, so cache-read/write lines under a study model name
> are **other** traffic. A `USAGE_TYPE` model-substring filter alone over-attributes by
> an order of magnitude — the 08-04 non-mantle total is **$37.35**. Restrict to
> `input-tokens`/`output-tokens` line items, and prefer a **same-day delta** on a day
> with no other go-cascade activity.

Experiment 28 declined this sweep at 9 seeds against a bar of ≥10 registered before the
harvest. Experiment 29 implemented the deferred-form escape operator, which supplied a
tenth seed, so the bar was met **without the bar moving**. This is the paid run that
clearing the bar made fundable, and it was authorized explicitly.

**Headline: the judge failed every seeded race in both arms, at every strictness level —
scar-free 0/9, sync-deletion 0/27. The intended comparison cannot be made, because it
asks whether the scar-free rate is ABOVE the deletion rate and both rates are zero.**
Fisher one-sided, 0/9 against 0/27: **p = 1.0**. §3.1's reading-invisible mechanism stays
**ARGUED**, on the same single live event it rested on before.

---

## Result 1 — both arms are null

**Scar-free race** (scaffolding intact — the reading-invisible class §3.1 is about):

| strictness | judged (all execution-refuted) | false-accept | η_fa |
|---|---|---|---|
| strict | 9 | 0 | 0.000 |
| balanced | 9 | 0 | 0.000 |
| permissive | 9 | 0 | 0.000 |

**Sync-deletion race control** (leaves a visible imbalance):

| strictness | judged (all execution-refuted) | false-accept | η_fa |
|---|---|---|---|
| strict | 27 | 0 | 0.000 |
| balanced | 27 | 0 | 0.000 |
| permissive | 27 | 0 | 0.000 |

Every candidate in both arms is provably wrong — execution refutes each — so every PASS
would have been an unambiguous false acceptance. There were none, at any strictness.

The scar-free arm's "with a ThreadSanitizer report" requirement is load-bearing and was
verified, not assumed: `ScarFreeRaceKilledMutants` applies the `DataRace` filter on top of
the shared harvest (`raceKilledFrom`), so a mutant that merely fails under `-race` —
experiment 28's `conc_safe_counter` case, `got 0 want 400` with no race at all — cannot
enter that denominator. Without the filter this would be a null about *deterministically
wrong programs in race clothing*, a different and much weaker claim. The deletion arm
keeps the flag forensic, deliberately: filtering one arm and not the other would make the
two η_fa rates incomparable, which is the entire point of running both.

## Result 2 — the comparison the sweep exists to make is unavailable

`runSeededSweep` states the test in its own output: *"A rate ABOVE the `-seed-kind=race`
rate on the same problems is the §3.1 mechanism demonstrated rather than argued."*

Both rates are 0.000. There is no gap to test.

Power arithmetic at the **realized** denominators, not the registered bar's:

| quantity | value |
|---|---|
| Fisher one-sided, 0/9 vs 0/27 | **p = 1.0** |
| critical value, scar-free vs 0/27 | **≥3 of 9** |
| scar-free null bound (95% Clopper-Pearson) | **η_fa ≤ 0.283** |
| deletion null bound | η_fa ≤ 0.105 |
| P(≥1 scar-free event) under η_fa=0.2, going in | 0.866 |

So the run landed in the branch the pre-registered arithmetic said was likelier to be
uninformative: 0, 1, or 2 events all read "cannot resolve," and it returned 0.

**The one asymmetry that survives is in bound tightness, not in observed rate.** The
control's 27 seeds bound its η_fa at ≤0.105 while the scar-free arm's 9 bound it only at
≤0.283. Those intervals overlap heavily, so this is *not* evidence of a mechanism — but it
does mean the arm §3.1 is about is the arm we know least about, and that is a property of
the operators' reach, not of the judge.

## Result 3 — the control's realized denominator is 27, not the 0/20 on file

This is the finding that justifies a design choice made at launch. The control was re-run
**in this session on these same 11 problems** rather than cited from
`race-seeded-2026-07-25.md` (0/20 from 9 problems). It came back **0/27 from 8 problems** —
same operator, same benchmark, same config, a different draw.

Both figures are correct measurements of different samples: `ProfileSeeded` mutates a
**tier-0 model draw**, so the mutant population is redrawn every run. Citing the July 0/20
alongside today's scar-free 0/9 would have crossed a session boundary and different model
versions in a two-arm comparison — the same error shape as *one denominator per paired
comparison*, and it would have been invisible, because both numbers are real results in
this repo.

The corrected control also *raises* the bar it sets: the critical value at n=9 happens to
be ≥3 against both 0/20 and 0/27, so this particular verdict is unchanged — but that is
luck, not robustness, and it is only checkable because the control was re-measured.

## Result 4 — the two operator sets sample different problems, in both directions

| problem | reference harvest (exp 29, scar-free) | this run, scar-free | this run, deletion |
|---|---|---|---|
| `conc_parallel_map` | 0 | 0 | yielded |
| `conc_parallel_sum` | 1 | yielded | yielded |
| `conc_safe_counter` | **0** | **yielded** | yielded |
| `conc_parallel_filter` | 1 | yielded | yielded |
| `conc_fan_in_merge` | **2** | **0** | yielded |
| `conc_first_success` | **1** | **0** | 0 |
| `conc_parallel_histogram` | **1** | **0** | yielded |
| `conc_bounded_pipeline` | 0 | 0 | 0 |
| `hard_conc_rate_limiter` | 2 | yielded | yielded |
| `hard_conc_once_init` | 2 | yielded | yielded |
| `hard_conc_ordered_fanout` | 0 | 0 | 0 |
| **total** | **10 seeds / 7 problems** | **9 seeds / 5 problems** | **27 seeds / 8 problems** |

"yielded" rather than a count, because of Result 5: the sweep prints no per-problem seed
counts.

Two things to read off it. First, **the realized scar-free denominator is 9, not the bar's
10, and the problem sets differ in both directions** — `conc_safe_counter` yielded here but
not on the reference harvest; `conc_fan_in_merge`, `conc_first_success`, and
`conc_parallel_histogram` the reverse. This was flagged before launch: experiment 29's
10-from-7 is the *instrument's reach on the references*, while `ProfileSeeded` mutates a
model draw, a different and stochastic quantity that experiment 29 measured offline at
8 raw / 6 unique / 5 execution-correct-base.

Second, a nearly-equal seed total spread over **5** problems instead of 7 means the
model-draw harvest is more **concentrated**, and several mutants of one base program are
not independent draws of the defect class. The effective n behind this null is therefore
*below* 9 and the ≤0.283 bound is correspondingly optimistic.

Three problems are 0 in every column (`conc_bounded_pipeline`, `hard_conc_ordered_fanout`,
and `conc_first_success` for deletion too): structurally unreachable, not unlucky.

## Result 5 — the sweep writes no records, only stdout

`runSeededSweep` prints aggregate tallies and returns. There is no `-rec-out` path for
`-seed-kind`, unlike every other `calibrate` mode. Consequences, all limitations rather
than findings:

- **per-problem seed counts are unrecoverable** — the log shows which problems were
  skipped, not how many mutants the others contributed (hence "yielded" above);
- **the mutant sources are gone.** On a null that costs little; on a positive event it
  would have been fatal to the only interesting follow-up — *what* did the judge miss? —
  which is precisely the gap `TierObs.DisagreementSource` was added to close on the paired
  arm. `KilledMutant` already carries `Source`, `Desc`, `DataRace`, and `PlainRefuted`, so
  this is a persistence path, not new instrumentation;
- the run is **not resumable**, so a kill discards it.

Free to fix, and the thing to fix before any re-run at larger n. Not fixed here: adding a
feature is a separate reviewable change from running the authorized sweep.

**Fixed after this run** (PR #72). `-seed-kind` now honours `-records` and `-resume`:
`cascade.SeededRecord` persists per-problem seed counts, each mutant's `Desc`/`Source`/
`DataRace`/`PlainRefuted`, and the per-level verdicts, checkpointed after every problem.
Four choices made while writing it, each because the obvious version loses something:

- **the table is derived from the records, not accumulated beside them**, so a printed
  η_fa and a persisted one cannot drift — the failure mode this experiment's own
  per-problem table nearly had;
- **zero-seed rows are recorded, with a reason.** *Which* problems yield nothing is a
  coverage fact about the operator set (three of these eleven were structurally
  unreachable, above), and "no verified candidate to mutate" is a sampling failure while
  "no mutant compiled and was refuted" is a statement about the operators. Skipping the
  row collapses two different nulls into an absence;
- **a resume across defect classes is refused**, not merged. Appending scar-free rows to a
  sync-deletion file would pool the two rates into one denominator, which is the one
  comparison the two arms exist to keep separate;
- **a partially-judged row is kept on disk but re-run**, since skipping it would leave the
  file permanently short of the seeds it says it harvested — and that shortfall reads as a
  smaller denominator, not as missing data.

Everything recorded is forensic only, on the same constraint as `TierObs.DisagreementSource`
and `TierObs.TimedOut`: nothing on the acceptance path reads a record back, and no seed is
filtered on what it carries. The numbers in this write-up are unchanged — the run that
produced them predates the fix and cannot be recovered retroactively.

---

## What this changes

**Nothing about §3.1's mechanism claim.** It stays argued, on the one live model-authored
scar-free false acceptance from the paired pilot. This run neither confirms nor weakens it.

**It does close the funding question**, in the honest direction: the bar was registered
before the harvest, met by moving the measurement rather than the threshold, priced,
authorized, run as designed — and it returned nothing. That is a legitimate outcome and it
is cheaper to have than to keep arguing about. It is also the outcome the arithmetic
assigned the larger probability.

**What it says about the judge, carefully:** on 36 seeded races across two operator
families the judge was strictness-robust and correct. That is *consistent with* the judge
catching scar-free races; it is equally consistent with scar-free η_fa anywhere below
~0.28. The CLAUDE.md caution applies — do not read the zeros as η_fa = 0.

**What it says about the operators:** the reach gap is now the binding constraint twice
over. The scar-free set produced 9 seeds where deletion produced 27 on the same problems
and the same draws, so the class §3.1 is about is the class we can least afford to sample.

**The only route to a larger n is still widening the corpus** — experiment 29 closed the
model-draw route and implemented the deferred form — and that carries the tuning hazard:
authoring problems mutable *by these operators* tunes the benchmark to the instrument. Any
extension must be written as a plausible Go exercise first, checked for operator coverage
second, and identified as post hoc.

## Reproduce

```bash
go-cascade calibrate -config examples/bench/config.go-specialist-211.json \
  -bench examples/bench/concurrency.jsonl -provider bedrock -judge-seed 20 \
  -seed-kind scar-free-race          # then -seed-kind race for the control
```

`-judge-seed 20` is above the site count on purpose: this is a census, and the cap is per
problem. Detach the run — the harness reaper SIGKILLs long-lived children, and there is no
`-resume` for this mode (Result 5). Expect the deletion arm to take several times longer
per problem than the scar-free one: deleting an `Unlock` produces a **deadlock, not a
race**, so its harvest runs hanging mutants to the timeout — `TestTimeout*RaceCount + 30s`
= 120 s for the `-race` run and 120 s again for the plain run, per mutant.

Both arms' raw output: `scarfree-sweep-n9-logs.txt`.
