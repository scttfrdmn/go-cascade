# The scar-free race sweep is declined on its own denominator: 6 seeds, and a null would bound η_fa only at 0.39 — 2026-08-05

Experiment 28. Offline, **$0**, ~11 min of local toolchain time. No model calls.

Issue #51 built the scar-free race operators (PR #56) to seed the one defect class §3.1
is actually about: racy code whose synchronization scaffolding is **intact and balanced**,
so a reading-only judge finds nothing missing on the page. The sync-deletion operator
cannot produce that shape — deleting `wg.Wait()` leaves a WaitGroup with `Add`/`Done` and
no wait — and it scored **20/20** against the judge precisely because imbalance is
visible without reasoning about interleaving.

The generator was merged; the measurement was not run. This experiment asks the free
question that has to come first: **can the operators reach a benchmark, and is the
resulting sample large enough to answer anything?**

> **Headline: the operator set nearly collapses on this corpus. 15 AST sites yield
> 6 usable seeds, 5 of them from a single operator, because the other two need
> constructs the benchmark does not contain. At n=6 the sweep can only detect a
> *large* effect (≥3 of 6 for p<0.05), while the likely outcome — a null, as with the
> 20/20 — would bound scar-free η_fa only at ≤0.393. The paid sweep is therefore
> declined: it would buy a number that cannot move the claim in either direction.
> §3.1's mechanism claim stays ARGUED, and the blocker is the corpus, not the
> generator and not the money.**

## Method

```bash
# AST site counts (instant, runs in the short suite):
go test ./internal/verify/ -run TestScarFreeOperatorCoverage -v
# Seed counts (compile + -race -count=3 per mutant, ~11 min):
#   ScarFreeRaceKilledMutants / RaceKilledMutants over each of the 11 references
python3 results/scarfree_coverage.py   # the power arithmetic
```

Sites come from `collectScarFreeRaceSites`. A site becomes a **seed** only if the mutant
compiles *and* is refuted by `go test -race -count=3` — the same filter both operator sets
share (`raceKilledFrom`), so the two rates stay comparable, which is the entire point of
having both.

## Result 1 — 15 sites, 6 seeds, and one operator carries the set

| operator | sites | seeds | fate |
|---|---|---|---|
| `wg.Wait()` → `defer wg.Wait()` | 8 | **5** | the only productive operator |
| `Lock/Unlock` → `RLock/RUnlock` | 6 | **0** | **dead**: every mutant fails to build |
| move last guarded stmt past `Unlock` | 1 | **1** | works, but there is one site |
| **total** | **15** | **6** | (sync-deletion, same problems: **17**) |

Six of eleven problems yield at least one seed.

## Result 2 — the downgrade operator is structurally unreachable, not unlucky

There is **no `sync.RWMutex` anywhere in `examples/bench/`**. All four mutex problems
(`conc_fan_in_merge`, `conc_parallel_histogram`, `hard_conc_rate_limiter`,
`hard_conc_once_init`) use plain `sync.Mutex`, so the downgrade produces:

```
./solution.go:40:7: mu.RLock undefined (type "sync".Mutex has no field or method RLock)
```

All 6 sites are discarded at the build filter — after paying for a compile each, which is
most of why the seed harvest takes 11 minutes. This is a property of the corpus that no
amount of sampling fixes.

## Result 3 — the `-race` filter earns its place, twice

Two sites look eligible and correctly produce nothing:

- **`conc_parallel_map`** has a `defer wg.Wait()` site and yields no seed. The operator's
  guard is "is there a statement after the `Wait`", and `return results` satisfies it — but
  for a slice return the deferred `Wait` still completes before the caller can observe
  anything, so the mutant does not race. Contrast `conc_parallel_sum`, which sums
  `partials` in a loop *after* the `Wait`, and `conc_fan_in_merge`, which calls
  `sort.Ints(out)` — both genuine races, both seeded.
- A **hand-check that went the other way** and is recorded because it nearly became a
  finding: moving `e.once.Do(...)` *into* `hard_conc_once_init`'s critical section passes
  `-race -count=3` (it over-serializes rather than racing). That is not what the operator
  does — it moves the *last guarded statement out*, which races on the shared map — and
  that mutant does seed. An AST site count alone would have gotten this wrong in both
  directions; only executing the mutants settles it.

## Result 4 — why 6 is not enough (the reason the sweep is declined)

The sweep's question is whether scar-free η_fa exceeds the sync-deletion rate of **0 of
20 judged**. Fisher one-sided against 0/17 on the same problems:

| scar-free accepts | rate | p | verdict |
|---|---|---|---|
| 0/6 | 0.000 | 1.0000 | cannot resolve |
| 1/6 | 0.167 | 0.2609 | cannot resolve |
| 2/6 | 0.333 | 0.0593 | cannot resolve |
| **3/6** | 0.500 | **0.0113** | significant |
| 4/6–6/6 | ≥0.667 | ≤0.0017 | decisive |

So **only a large effect is detectable** — and the hypothesis under test is that this
class is *subtler* than the visible one, i.e. precisely the regime where a small
denominator fails. Worse, the likely outcome is a null: the judge caught 20/20
scar-bearing races, and 0/6 gives a 95% upper bound of **η_fa ≤ 0.393**. That does not
distinguish "the judge catches scar-free races" from "six seeds cannot tell you anything
below 39%."

Both branches buy nothing. Hence declined — the first experiment in this study stopped by
a **power** calculation rather than by a result.

## What this establishes

1. **The scar-free generator is correct and the corpus is the blocker.** The operators
   produce genuinely balanced, `-race`-refuted mutants (asserted by
   `TestScarFreeMutantsKeepScaffoldingBalanced` and
   `TestScarFreeRaceKilledMutantsAreRaceRefuted` on a synthetic fixture). They simply
   cannot find enough eligible constructs in 11 hand-written problems.
2. **§3.1's reading-invisible mechanism stays ARGUED, not confirmed.** Unchanged from
   before this experiment — but now the obstacle is quantified rather than assumed, and the
   quantity says what would have to change.
3. **A seeded sweep over these 11 problems must not be run and reported as η_fa.** The
   existing "NO SEEDS" guard in `runSeededSweep` protects only the zero-denominator case;
   n=6 is worse, because it prints a table that looks like a measurement.
4. **`defer wg.Wait()` needs a *read* after the wait, not merely a statement.** A returned
   slice is not an observation the mutant can lose. This is a real (small) weakness in the
   operator's guard, documented rather than fixed, since fixing it would not change the
   denominator materially.

## What this does not establish

- **Nothing about the judge.** No candidate was judged and no model was called. Any
  reading of this as evidence about η_fa would be exactly the error the experiment exists
  to avoid.
- **Not that the scar-free class is rare in the wild.** It is rare *in this benchmark*,
  which was hand-written before these operators existed. The live pilot's one confirmed
  judge false-acceptance was a model-authored scar-free race, so the class is clearly
  reachable by models even where the operators struggle.
- **Not that the operator set is complete.** Loop-variable capture is obsolete (Go 1.22
  per-iteration variables — pinned by `TestLoopVarCaptureDoesNotRaceOnThisToolchain`), but
  other scar-free shapes exist: a missing `atomic` on a counter guarded elsewhere, a
  channel send moved outside a `select`, a `RWMutex` read lock held across a write in a
  *different* method. None are implemented.

## If this is to be reopened

Reaching a denominator where a null is informative (0 events at n=23 bounds η_fa ≤0.122;
n=40 ≤0.072) means adding concurrency problems with `sync.RWMutex` and with
multi-statement critical sections, which revives the two dead operators.

**The trap to avoid, stated in advance:** hand-authoring problems chosen to be mutable *by
these operators* tunes the benchmark to the instrument, and a 20-seed sweep over a corpus
built for the seeder is not the same measurement as a 20-seed sweep over an independent
one. Any extension has to be written as a plausible Go exercise first and checked for
operator coverage second, and the write-up must name which problems were added after the
operators existed. `TestScarFreeOperatorCoverageOnTheConcurrencyBenchmark` pins the current
counts, so such an extension **breaks the test on purpose** — that failure is the signal to
re-measure the seed count and re-price the sweep.

## Reproduce

```bash
go test ./internal/verify/ -run TestScarFreeOperatorCoverage -v   # sites, instant
python3 results/scarfree_coverage.py                             # power arithmetic
```

Seed counts: `ScarFreeRaceKilledMutants(ctx, r, ref, refTests, "", 5, 3, 60s)` over the 11
`examples/bench/concurrency.jsonl` references, ~11 min, no credentials.
