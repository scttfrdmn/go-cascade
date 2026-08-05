# The deferred-escape operator: the declined sweep now clears its bar (experiment 29)

**Date:** 2026-08-05 · **Cost: $0** (offline; no Bedrock calls) ·
**Supersedes the *decision* in [`scarfree-coverage-n11.md`](scarfree-coverage-n11.md),
not its measurements**

Experiment 28 declined a ~$3 seeded scar-free race sweep at **9 seeds against a bar of
≥10 registered before the harvest**, and its write-up named three reopening routes in
cost order. This experiment ran the two free ones. Route 2 — implement the deferred form
of the escape operator — supplied the tenth seed, so **the bar is met without the bar
moving**. Route 1 — harvest from model draws instead of references — was measured and is
**closed**: it does not raise the count.

Two pre-existing defects in the *shipped* operator set were found on the way, both of
which had been making the arm weaker than it was documented to be.

---

## Result 1 — the deferred form of the escape operator was never implemented

`collectScarFreeRaceSites` moved a shared write out of a critical section only where the
`Unlock` was an *explicit statement*. It skipped every

```go
mu.Lock()
defer mu.Unlock()
```

site — which is the dominant Go idiom, and the reason the escape operator had exactly
**one** site on an 11-problem concurrency benchmark while `defer-wait` had 8. The
deferred form (`escape-defer`) moves the last statement of the covered region past the
lock and converts the `defer` to a plain call, keeping the pair balanced on the page.

| | sites | seeds |
|---|---|---|
| `defer-wait` | 8 | 4 |
| `downgrade` (RWMutex) | 6 | 4 |
| `escape` (explicit Unlock) | 1 | 1 |
| **`escape-defer` (new)** | **1** | **1** |
| **total** | **16** | **10** |

Seeds by problem: `conc_parallel_sum` 1, `conc_parallel_filter` 1, `conc_fan_in_merge` 2,
`conc_first_success` 1, `conc_parallel_histogram` 1, `hard_conc_rate_limiter` **2**,
`hard_conc_once_init` 2 — 7 of 11 problems contributing, as before.

The count moved from 9 to 10 and **the bar was left at 10**. That distinction is the
whole point of registering it in advance, and it was enforced mechanically rather than by
anyone re-reading the write-up: `TestScarFreeSeedCountOnTheConcurrencyBenchmark` was
pinned at 9 and **failed** when the operator landed.

Power at n=10 against the sync-deletion control (0/20), from
[`scarfree_coverage.py`](scarfree_coverage.py):

- critical value **≥3 of 10** (unchanged from n=9 — the Fisher threshold is a step
  function, which is why one seed buys more than it looks like it should)
- power at η_fa=0.3: **0.617**, up from 0.537
- P(≥1 event) at η_fa=0.2: **0.893**
- a null bounds η_fa at **≤0.259** (95%), down from 0.283

**Clearing the bar is not the same as being well-powered.** What n=10 buys is an
*existence proof* at ~89% under η_fa=0.2, not a rate. A null still resolves little.

## Result 2 — the tenth seed is the first that *needs* the `-race` rung

Experiment 28's Result 3 was that **all 9 seeds were also refuted by a plain (no `-race`)
test run**, so the rung was not load-bearing for any of them — the sweep could only have
measured whether a reader notices a program that already fails deterministically. That
was the sharpest caveat on seed quality in the whole document.

The `escape-defer` seed on `hard_conc_rate_limiter.Refill` is the exception: **9 of 10 are
plain-refuted, and it is not.** Undeferring the `Unlock` widens the window enough that the
ordinary run passes and only ThreadSanitizer objects. So the arm now contains at least one
seed of exactly the shape §3.1 is about — a program a reader must reason about
interleavings to reject, and which a deterministic test run lets through.

This is a bigger change to *what the sweep would measure* than the count going 9 → 10.

`PlainRefuted` remains **recorded and never filtered on** — filtering one arm and not the
other makes the two η_fa rates incomparable, which is why `raceKilledFrom` is shared. The
asymmetry is asserted per operator (`wantRaceOnly`), not as a total: a total of 1 could be
satisfied by any operator drifting into the bucket, and the claim is specifically that
*this* operator produces `-race`-only seeds.

## Result 3 — route 1 (model-draw population) is measured and closed

`ProfileSeeded` mutates a **tier-0 model draw**, never a reference, so the 9/10 above is a
proxy for a population the sweep does not use. Experiment 28 called harvesting from model
draws "nearly free, right population" and listed it first. It is free — experiment 25
retained candidate sources — and it does **not** raise the count:

| denominator | seeds |
|---|---|
| raw rows | 8 |
| unique programs | 6 |
| unique programs with an **execution-correct base** | 5 |

9 recorded rows are only 7 unique programs: `conc_safe_counter` appears **3× byte-identically**.
And `ProfileSeeded` does not check that the program it mutates is correct to begin with, so
mutants of an already-wrong base would be counted as seeds — a mutant of a broken program
is not a seeded defect, it is two defects.

**5–8 depending on which denominator you pick is not clearly above 10.** Report the range,
not the flattering end. What the harvest *does* establish: all 7 unique bases are
race-clean under `-race`, so the operators cause the races rather than exposing pre-existing
ones.

## Result 4 — two defects in the shipped operators, both self-flattering

Neither was caught by any existing test, and both made the arm weaker than documented.

**(a) A formatting artifact is a scar.** `go/printer` lays a block out from its statements'
*recorded source lines*, not their slice order. Any reordering operator makes those lines run
backwards and the printer emits a **blank line at exactly the mutation site** — and `gofmt`
does not remove it, because gofmt preserves author blank lines between statements. Measured
before the fix: 3 of 17 sites carried a positional tell, all from the reordering operators
(`downgrade` and `defer-wait` edit in place and never did). An arm whose premise is "nothing
on the page is missing" cannot leave a blank line where the edit happened.

Fixed with `clearPositions`, a reversible position-zeroing helper. **Scope it to the
statements that moved, never to the enclosing block:** clearing the whole block makes the
printer re-lay-out formatting the mutation never touched — on `hard_conc_once_init` it
collapsed a three-line `once.Do(func(){...})` to one line and dropped an author blank line
elsewhere, 52 printed lines in and **49** out. Scoped to the moved statements it is 52 → 52.
A three-line scar is not an improvement on a one-line one. Now **0 of 16 sites** move the
blank-line count, guarded by `TestScarFreeMutantsLeaveNoFormattingScar` — which compares
against the printer's **round-trip of the original**, not the source, because `printer.Fprint`
retabs struct fields and the source is not its own fixed point.

`clearPositions` is confined to the two scar-free reordering operators. `collectRaceSites`
and `collectSites` are untouched, so comparability with the scar-bearing arm holds.

**(b) The deferred-escape operator manufactured a deadlock.** Undeferring an `Unlock` only
unlocks on the fall-through path, so a `return` inside the covered region leaks the lock and
the mutant **hangs**. Wrong defect class on two counts: a timeout is the one refutation whose
cause can be external to the candidate (a hang is machine load's signature too), and a `Lock`
with a `return` before its `Unlock` is a *visible* imbalance — the deletion arm's territory,
which is precisely what this operator set exists to avoid.

Found by dumping both candidate mutants and reading them, not by the DATA-RACE filter: the
first mutant on `hard_conc_rate_limiter.Allow` hangs, and a hang is not a race, so the filter
would have silently dropped it and reported one seed either way. Vetoed by `exitsIn`
(16 sites, down from 17). **A `return` inside a `func` literal is not an exit** — it exits the
literal, not the enclosing critical section — and vetoing on it would reject correct sites in
an operator set that has little enough reach already.

---

## What this changes

**In `scarfree-coverage-n11.md`:** the decision (DECLINE at 9) and Result 3 (all seeds also
plain-refuted). Its two retractions and its power arithmetic stand. That document is left as
written — the decline was correct on the evidence it had, and rewriting it would erase the
sequence that makes the bar meaningful.

**Still argued, not demonstrated:** §3.1's reading-invisible mechanism. This experiment
raises the *instrument's* reach; it does not run the sweep. Nothing has been spent.

**The next step is a price, not a run.** The bar is met, so the sweep is fundable rather than
declined — but a bar clearing does not authorize spend. Quote against a past bill
(`results/README.md` §"A correction to every cost figure on this page"), not a per-token
estimate, and get it approved.

## Reproduce

```bash
go test ./internal/verify/ -run 'TestScarFree|TestDeferredEscape' -v   # ~24 s, needs the toolchain
python3 results/scarfree_coverage.py                                    # power arithmetic at n=10
```

Pinned by `TestScarFreeOperatorCoverageOnTheConcurrencyBenchmark` (16 sites),
`TestScarFreeSeedCountOnTheConcurrencyBenchmark` (10 seeds, per problem, per operator, and the
`-race`-only asymmetry), `TestScarFreeMutantsLeaveNoFormattingScar`, and
`TestDeferredEscapeVetoesControlFlowExits`.
