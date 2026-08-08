# The scar-free race sweep is declined at 9 seeds against a bar of 10 — and the first pass of this experiment was wrong twice, both times in its own favour — 2026-08-05

Experiment 28. Offline, **$0**, ~20 s of toolchain time per harvest. No model calls.

> **⚠ The DECISION in this document has been superseded — the measurement has not.**
> Experiment 29 ([`deferred-escape-n11.md`](deferred-escape-n11.md)) implemented the
> second of the two free routes listed at the bottom of this page: the deferred form of
> the escape operator, which had been skipping every `Lock(); defer Unlock()` site. It
> supplies a **tenth** seed, so the count is now 10 against this page's bar of 10 and the
> sweep is **fundable rather than declined**. The bar did not move; the measurement did.
> Two other findings here are also revised there: **Result 3's "all 9 seeds are also
> refuted without `-race`" is now 9 of 10 on a 12-core machine and 8 of 10 on a 2-core
> one** (the new seed is the first that genuinely needs the rung; the count is
> machine-dependent because `PlainRefuted` is the outcome of *running* a racing program —
> see the 2026-08-06 correction in `deferred-escape-n11.md`), and the "harvest from model
> draws" route is **measured and closed** —
> 8 raw / 6 unique / 5 from execution-correct bases, not clearly above 10.
> Everything below is left exactly as written. The decline was correct on the evidence it
> had, and rewriting it would erase the sequence that makes a pre-registered bar mean
> anything.

Issue #51 built the scar-free race operators (PR #56) to seed the one defect class §3.1
is actually about: racy code whose synchronization scaffolding is **intact and balanced**,
so a reading-only judge finds nothing missing on the page. The sync-deletion operator
cannot produce that shape — deleting `wg.Wait()` leaves a WaitGroup with `Add`/`Done` and
no wait — and it scored **20/20** against the judge precisely because imbalance is
visible without reasoning about interleaving.

The generator was merged; the measurement was not run. This experiment asks the free
question that has to come first: **can the operators reach a benchmark, and is the
resulting sample large enough to answer anything?**

> **Headline: 15 AST sites yield 9 usable seeds — mutants that compile and draw a real
> ThreadSanitizer report. Against a bar of ≥10 registered *before* the harvest, that is
> one short, so the paid sweep stays declined. But the honest framing is CLOSE, not
> CLEAR: at n=9 an existence proof has 87% probability under η_fa=0.2, the critical
> value is 3, and a null would bound η_fa at ≤0.283. §3.1's mechanism stays ARGUED.**

> **Read the retraction below before citing any of this.** The first pass of this
> experiment reported 6 seeds against a control of 0/17 and concluded the operator set
> "nearly collapses on this corpus" with the blocker being "the corpus, not the generator."
> Two of those three claims were wrong, and both errors pushed toward declining.

## Two corrections, both self-favouring

### (1) The RWMutex downgrade operator was broken, not the corpus

**Retracted:** "the `RWMutex` downgrade is **structurally dead**: there is no
`sync.RWMutex` anywhere in `examples/bench/`, so all 6 of its mutants fail to build.
This is a property of the corpus that no amount of sampling fixes."

All 6 mutants did fail to build, with `mu.RLock undefined (type "sync".Mutex has no field
or method RLock)`. The cause was the operator: it rewrote `mu.Lock()` → `mu.RLock()` and
left `var mu sync.Mutex` untouched, on the reasoning recorded in its own doc comment —
"only compiles on an RWMutex, **which the build filter enforces rather than a type
check**." Deferring a type constraint to the compiler turns an unreachable operator into
an empty result, and an empty result reads as a finding.

Co-mutating the declaration (`sync.Mutex` → `sync.RWMutex`) is the same single conceptual
edit — a read lock held over a write — and it yields **4 seeds**. It is also *more*
scar-free than the operators that survived: every call is present, paired, and idiomatic;
the reader must independently realise the guarded statement is a write.

**Consequence: 6 seeds → 9, and "one operator carries the set" is retracted.** The
downgrade operator is now co-equal with `defer-wait` at 4 seeds each.

### (2) The control denominator was the wrong experiment's

**Retracted:** "Fisher one-sided against 0/17 **on the same problems**."

17 is the **logic**-defect arm over **6 different** problems (`seeded-2026-07-25.md`). The
sync-deletion arm on these 11 is **0/20** from 9 problems (`race-seeded-2026-07-25.md`).
The first write-up set up the comparison as "0 of **20** judged" and computed against
**0/17** in the next sentence; the script carried both constants and used the wrong one.

Both numbers are real results in this repo, which is exactly why the swap survived a
read-through. It moved the critical value at n=6 from **2 to 3** — and 2/6 was the row
labelled "cannot resolve," which against the correct control is significant at p=0.046.
This repo already recorded the general form of this error: *a paired comparison must print
one denominator.*

## Method

```bash
go test ./internal/verify/ -run TestScarFreeOperatorCoverage -v      # AST sites, instant
go test ./internal/verify/ -run TestScarFreeSeedCount -v             # seeds, ~20 s
python3 results/scarfree_coverage.py                                 # power arithmetic
```

Sites come from `collectScarFreeRaceSites`. A site becomes a **seed** only if the mutant
compiles *and* `go test -race -count=3` produces a **DATA RACE** report. Both counts are
now pinned by tests; the first pass pinned only sites, which is why a wrong seed count
survived a green suite.

## Result 1 — 15 sites, 9 seeds, no dominant operator

| operator | sites | seeds |
|---|---|---|
| `wg.Wait()` → `defer wg.Wait()` | 8 | 4 |
| `Lock/Unlock` → `RLock/RUnlock` + declaration → `RWMutex` | 6 | **4** |
| move last guarded stmt past `Unlock` | 1 | 1 |
| **total** | **15** | **9** |

Seven of eleven problems yield at least one seed. Control: sync-deletion **0/20**.

## Result 2 — the DATA RACE filter is a filter, and it caught a miscount

The scar-free arm requires an actual ThreadSanitizer report, not merely a `-race` failure.
`conc_safe_counter`'s deferred-`Wait` mutant fails under `-race` with `got 0 want 400` and
**no race at all** — it is a deterministic wrong answer wearing this operator's clothes.
The first pass counted it as a race seed. It is now excluded (`KilledMutant.DataRace`,
enforced in `ScarFreeRaceKilledMutants`), which is why the count is 9 rather than 10 and
why the bar is missed. Excluding it is right: a seed that tests nothing about
race-blindness inflates the denominator of an experiment about race-blindness.

The deletion arm keeps the flag **forensic only**. Filtering one arm and not the other
would make the two η_fa rates incomparable, and comparing them is the entire point of
having both (`raceKilledFrom`'s contract).

## Result 3 — all 9 seeds are also refuted WITHOUT `-race`, which is a caveat on seed quality

Every one of the 9 is refuted by the plain (no `-race`) test run at `-count=3`. Two
distinct readings, and only the first is established:

- **Established, about the ladder:** the `-race` rung is not load-bearing for *these*
  seeds. An ordinary run already refutes them, so they do not exercise the rung that
  caught the study's only confirmed judge false-acceptance.
- **Not established, about the judge:** it does *not* follow that a reader would notice. A
  deferred `wg.Wait()` with balanced `Add`/`Done` is scar-free on the page whether or not
  the resulting failure is deterministic — reader-visibility is a property of the source,
  and determinism is a property of the execution.

What it does mean is that these seeds test "judge vs a deterministically wrong answer"
more than "judge vs an interleaving it must reason about," which is weaker than the arm
was designed to be. Recorded (`KilledMutant.PlainRefuted`), deliberately not filtered on.

## Result 4 — the power arithmetic at the corrected n

Fisher one-sided against the sync-deletion **0/20**:

| scar-free accepts | rate | p | verdict |
|---|---|---|---|
| 0/9 | 0.000 | 1.0000 | cannot resolve |
| 1/9 | 0.111 | 0.3103 | cannot resolve |
| 2/9 | 0.222 | 0.0887 | cannot resolve |
| **3/9** | 0.333 | **0.0230** | significant |
| 4/9 | 0.444 | 0.0053 | decisive |

Critical value **≥3 of 9**. A null (0/9) bounds η_fa at **≤0.283**. Power at the critical
value is 0.262 at η_fa=0.2 and 0.537 at 0.3 — still low. But P(≥1 event) is **0.866** at
η_fa=0.2, and one event is an **existence proof** that needs no p-value: §3.1's mechanism
claim currently rests on n=1, and a second confirmed scar-free false acceptance would
move it regardless of significance.

## Result 5 — the decision, and why it is not obviously right

**Registered bar, set before the corrected harvest ran: run the sweep iff ≥10 seeds
survive the DATA RACE filter. Harvested 9. Declined.**

The bar is honoured because a bar moved after seeing the data is not a bar. But the
strongest argument *against* this decision belongs in the record: for ~$3, an experiment
with an 87% chance of an existence proof on the study's weakest claim is not obviously a
bad buy, and "both branches buy nothing" — the first pass's phrasing — is too strong at
n=9. What the bar buys is that the decision was not made by looking at 9 and reasoning
backwards.

## Result 6 — the seed count is measured on the wrong population

`ProfileSeeded` (`internal/cascade/judge.go:441-448`) draws a **tier-0 model candidate**,
clusters it, and mutates the cluster winner. It never touches a reference. So **9 is a
reference-only proxy** for a denominator nobody has measured.

The direction of the error is unknown but the population that matters is the model's: the
study's one confirmed live scar-free false acceptance was **model-authored**. Harvesting
from tier-0 draws costs one cheap sample per problem and could clear the bar on its own.
That is the cheapest route to reopening this, and it is cheaper than authoring problems.

## What this establishes

1. **The generator had a reach bug that the corpus took the blame for.** Fixed; the
   downgrade operator now contributes 4 of 9 seeds. Documented in `mutation.go` as a
   measurement bug, not a shortcut, so it is not reintroduced.
2. **§3.1's reading-invisible mechanism stays ARGUED.** Unchanged in substance from before
   this experiment — but the obstacle is now quantified, and the quantity says the gap to
   informative is one to a few seeds, not an order of magnitude.
3. **A sweep at n=9 must not be reported as η_fa without its critical value.** At 0, 1, or
   2 events the result is "cannot resolve," and a table of rates would look like a
   measurement.
4. **`defer wg.Wait()` needs a *read* after the wait, not merely a statement.** The three
   non-productive `defer-wait` sites (`conc_parallel_map`, `conc_parallel_histogram`'s
   second site, `hard_conc_ordered_fanout`) are those whose only post-`Wait` statement is a
   bare `return`: a returned slice is not an observation the mutant can lose, because the
   deferred `Wait` still completes before the caller sees anything. Documented, not fixed —
   fixing it removes dead sites without adding seeds.

## What this does not establish

- **Nothing about the judge.** No candidate was judged and no model was called. Any
  reading of this as evidence about η_fa would be exactly the error the experiment exists
  to avoid.
- **Not that the scar-free class is rare in the wild.** It is not especially rare even in
  this benchmark now — 7 of 11 problems yield a seed. The live pilot's one confirmed judge
  false-acceptance was a model-authored scar-free race.
- **Not that the operator set is complete.** Loop-variable capture is obsolete (Go 1.22
  per-iteration variables — pinned by `TestLoopVarCaptureDoesNotRaceOnThisToolchain`), and
  the deferred-form escape (`Lock(); defer Unlock(); A; B` → move `B` out) is not
  implemented: `collectScarFreeRaceSites` skips deferred Unlocks, which `syncCall`'s own
  doc comment calls "the dominant Go idiom." Other scar-free shapes exist too — a missing
  `atomic` on a counter guarded elsewhere, a channel send moved outside a `select`.
- **Not that 9 is the sweep's denominator.** See Result 6: the sweep mutates model draws.

## If this is to be reopened

In cost order:

1. **Harvest from tier-0 model draws** (Result 6). Nearly free, measures the right
   population, and plausibly clears the bar by itself.
2. **Implement the deferred-form escape operator.** Free, and `hard_conc_rate_limiter`
   alone has three `Lock(); defer Unlock()` sites currently skipped.
3. **Widen the corpus** with multi-statement critical sections. Note this now revives only
   the *escape* operator; the downgrade one is alive.

**The trap to avoid, stated in advance:** hand-authoring problems chosen to be mutable *by
these operators* tunes the benchmark to the instrument, and a 20-seed sweep over a corpus
built for the seeder is not the same measurement as one over an independent corpus. Any
extension has to be written as a plausible Go exercise first and checked for operator
coverage second, and the write-up must name which problems were added after the operators
existed. Routes 1 and 2 are immune to this hazard — neither touches the benchmark — which
is another reason they come first.

`TestScarFreeSeedCountOnTheConcurrencyBenchmark` pins the seed count and
`TestConcurrencyRefIDsMatchTheBenchFile` pins the problem count, so any of these
**breaks the suite on purpose** — that failure is the signal to re-price the sweep.

## Reproduce

```bash
go test ./internal/verify/ -run TestScarFreeOperatorCoverage -v   # sites, instant
go test ./internal/verify/ -run TestScarFreeSeedCount -v          # seeds, ~20 s
python3 results/scarfree_coverage.py                              # power arithmetic
```

No credentials. The superseded first pass reported 6 seeds against 0/17; its numbers are
retracted above rather than kept as a variant file, because unlike experiment 19's two
draws they are not a second sample of the same quantity — they are a wrong measurement of
it.
