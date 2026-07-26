# Race-seeded dangerous-mode test — 2026-07-25

Targets the study's one confirmed live blind spot. The seeded logic test
(`seeded-2026-07-25.md`) found the judge caught all 17 single-edit logic
defects, but the only confirmed live false acceptance across the whole study was
a **data race** (paired pilot, `pilot2-paired-2026-07-25.md`). This run seeds
race defects specifically — deleting a synchronization statement from a correct
concurrent solution — and judges each at every strictness level. 11 concurrency
problems, α=0.10, zero errors; 9 problems yielded race mutants (2 had none).

## Result

| strictness | judged (all race-refuted) | false-accept | η_fa rate |
|------------|---------------------------|--------------|-----------|
| strict     | 20                        | 0            | 0.000     |
| balanced   | 20                        | 0            | 0.000     |
| permissive | 20                        | 0            | 0.000     |

The judge FAILED all 20 seeded race defects, at every strictness level.

## Why this does NOT contradict the pilot's η_fa — a real limitation of the operator

At face value this says "the judge catches races", which conflicts with the
paired pilot where the judge *passed* a racy `conc_parallel_map`. The
reconciliation is a methodological subtlety worth stating plainly:

**The race-seeding operator deletes a synchronization statement, which leaves a
visible structural scar.** Deleting `wg.Wait()` produces a function that declares
`var wg sync.WaitGroup`, calls `wg.Add(1)` and `wg.Done()`, and then *never waits
on the group*. To a competent reviewer that is a glaring tell — "you built a
WaitGroup and never called Wait" — so the judge FAILs it by spotting the dangling
scaffolding, not by reasoning about the interleaving. Same for a deleted
`Unlock` (a `Lock` with no matching `Unlock`, an obvious imbalance).

The pilot's false-accepted race was **not** of this form: the model *wrote* an
unsynchronized shared write from scratch — a self-consistent, natural-looking
concurrent function with no missing-obvious-call tell, that simply happened to
race. That is the genuinely reader-invisible case, and this mutation operator
does not reproduce it.

So the honest reading is:

- **The judge reliably catches races that leave a syntactic imbalance** (missing
  Wait, unmatched Lock) — 20/20 here, strictness-robust.
- **This says little about races with no such tell**, which is the class the
  pilot showed it can miss. Sync-*deletion* is the wrong generator for that
  class; it always leaves a scar.

## What would actually test the reader-invisible race class

The defect needs to look self-consistent. Options, none of which sync-deletion
provides:

1. **Model-authored wrong candidates.** Ask a weaker/among-tier model for a
   concurrent solution and keep the ones execution refutes under `-race` but that
   have no missing-sync tell (as the pilot's did). This is closest to reality but
   not deterministic.
2. **A scar-free race operator.** e.g. narrow a critical section (move a shared
   write outside an existing `Lock`/`Unlock` rather than deleting the pair), or
   swap a value-capture for a loop-variable capture in a goroutine. These keep the
   sync scaffolding intact, so the code reads as balanced while still racing.
3. **A curated seed set** of hand-written, natural-looking racy solutions.

## Where this leaves the study

The claim from `seeded-2026-07-25.md` stands and is now bracketed:

- For **logic defects** and for **races with a structural tell**, this judge is
  reliable and strictness-robust (η_fa = 0 across 37 seeded wrong candidates).
- The **one** defect class where η_fa > 0 was observed live remains the
  **scar-free race** (pilot #8). This run did not manage to seed that class — a
  limitation of sync-deletion, not evidence the class is safe.

Net: the §3.1 danger is real (observed once, live) and narrow, but pinning down
*how* narrow needs a scar-free race generator. That is the honest next step, and
it is a harder generator-design problem than the deletion operator built here.

Live spend this session: roughly $23–27 across six experiments and diagnosis.
