#!/usr/bin/env python3
"""Power arithmetic for the seeded scar-free race sweep (experiments 28, 29 and 30).

THE SWEEP HAS NOW RUN (experiment 30) AND BOTH ARMS ARE NULL: scar-free 0/9,
sync-deletion 0/27, at every strictness level. Fisher p = 1.0. This file is therefore
the record of how a spend decision was priced, plus (at the bottom) what the spend
bought — which was a null that the arithmetic here assigned the larger probability.
Read results/scarfree-sweep-n9.md for the run itself.


    python3 results/scarfree_coverage.py

The scar-free race operators (internal/verify/mutation.go, PR #56) exist to seed the
one defect class §3.1 is actually about: racy code whose synchronization scaffolding is
intact, so a reading-only judge finds nothing missing. The sync-deletion operator cannot
produce that shape — it leaves a WaitGroup with no Wait — and scored 20/20 against the
judge precisely because imbalance is visible on the page.

Measured coverage on the 11-problem concurrency benchmark: **16 AST sites, 10 usable
seeds** (compile + ThreadSanitizer-confirmed race). This script asks whether that can
answer the question the sweep exists to answer, and the answer is now yes-at-the-margin:
exactly the bar registered before experiment 28's harvest ran.

THE VERDICT IN THIS FILE FLIPPED, and how it flipped is the point. Experiment 28
harvested 9 against a bar of >= 10 registered in advance, and DECLINED — see
results/scarfree-coverage-n11.md, which is left standing as written. Experiment 29 then
implemented one of the two free routes that write-up named (the deferred-form escape
operator, which had been skipping `Lock(); defer Unlock()` — the dominant Go idiom) and
it supplied the tenth seed. The bar was not moved; the measurement was. Nothing here was
re-tuned after seeing a result: the Go test pinned at 9 FAILED when the operator landed,
which is how the flip was noticed at all.

THIS SCRIPT'S FIRST VERSION GOT TWO INPUTS WRONG, both in the direction that made
declining look better, and both are worth reading before trusting any number here:

  1. The control was 0/17 labelled "same problems". 17 is the *logic*-defect arm over 6
     DIFFERENT problems (results/seeded-2026-07-25.md). The sync-deletion arm on these 11
     is 0/20 (results/race-seeded-2026-07-25.md). Using 17 moved the critical value from
     2 to 3 of 6 and understated power 3-7x.
  2. The seed count was 6 because the RWMutex downgrade operator produced nothing, which
     was reported as a fact about the corpus ("no sync.RWMutex in examples/bench"). It was
     a bug: the operator rewrote the call sites and left the declaration a sync.Mutex, so
     every mutant failed to build. Co-mutating the declaration yields 4 downgrade seeds.

No scipy: Clopper-Pearson by bisection on the exact tail sum, Fisher by enumerating the
hypergeometric tail. At n=10 a normal approximation is meaningless.
"""

from math import comb


def cp_upper(k: int, n: int, conf: float = 0.95) -> float:
    """Exact one-sided upper confidence bound on a binomial rate given k events of n."""
    lo, hi = 0.0, 1.0
    for _ in range(200):
        m = (lo + hi) / 2
        tail = sum(comb(n, i) * m**i * (1 - m) ** (n - i) for i in range(k + 1))
        if tail > 1 - conf:
            lo = m
        else:
            hi = m
    return (lo + hi) / 2


def fisher_one_sided(a: int, b: int, c: int, d: int) -> float:
    """P(scar-free accepts >= a) with margins fixed. Exact hypergeometric tail."""
    n1, n2, k = a + b, c + d, a + c
    lo, hi = max(0, k - n2), min(n1, k)
    tot = sum(comb(n1, i) * comb(n2, k - i) for i in range(lo, hi + 1))
    top = sum(comb(n1, i) * comb(n2, k - i) for i in range(a, hi + 1))
    return top / tot


def binom_tail(k: int, n: int, p: float) -> float:
    """P(X >= k) for X ~ Binomial(n, p)."""
    return sum(comb(n, i) * p**i * (1 - p) ** (n - i) for i in range(k, n + 1))


def crit_value(n: int, control_k: int, control_n: int, alpha: float = 0.05) -> int:
    """Smallest number of events at n that clears alpha against the control."""
    for a in range(n + 1):
        if fisher_one_sided(a, n - a, control_k, control_n) < alpha:
            return a
    return n + 1


SCARFREE_SEEDS = 10

# The control is the SYNC-DELETION arm on these same 11 concurrency problems:
# results/race-seeded-2026-07-25.md, 20 race mutants from 9 problems (2 yielded
# none), 0 false acceptances at each of three strictness levels.
#
# It is NOT 17. That is results/seeded-2026-07-25.md — the *logic*-defect arm, 17
# mutants over 6 different problems. The first pass of this script used 17 and
# labelled it "same problems", which is the arm-comparison equivalent of citing the
# wrong denominator: it moves the critical value from 2 to 3 of 6 and so understates
# the sweep's power by 3-7x in the plausible range. Both numbers are real results in
# this repo, which is exactly why the swap survived a read-through.
DELETION_SEEDS = 20

# EXPERIMENT 30 RAN THE SWEEP. Both arms are NULL: scar-free 0/9, sync-deletion 0/27,
# at all three strictness levels (results/scarfree-sweep-n9.md). Fisher 0/9 vs 0/27 is
# p = 1.0 — the comparison this script prices asks whether the scar-free rate is ABOVE
# the deletion rate, and both rates are zero, so there is no gap to test.
#
# Two realized numbers differ from the ones priced above, and both matter more than the
# verdict did:
#
#   1. The scar-free denominator came back 9, not 10, from 5 problems rather than 7.
#      SCARFREE_SEEDS below is the REFERENCE harvest — the instrument's reach — and
#      ProfileSeeded mutates a tier-0 MODEL DRAW, so the run's yield is a separate
#      stochastic quantity. It is left at 10 deliberately: it is the pinned coverage
#      measurement that the Go test asserts, not a record of what the sweep drew.
#   2. The control came back 0/27 from 8 problems, NOT the 0/20 on file from
#      2026-07-25. Same operator, same benchmark, same config, different draw. The
#      critical value at n=9 is >=3 against both, so the verdict is unchanged — but
#      that is luck, and it is only checkable because the control was re-measured in
#      the same session rather than cited across one.
#
# DELETION_SEEDS stays 20 so this script keeps reproducing the arithmetic that priced
# the DECISION. Read experiment 30 for what the run returned.
DELETION_SEEDS_EXP30 = 27
SCARFREE_SEEDS_EXP30 = 9

# The bar was registered BEFORE experiment 28's corrected harvest ran, precisely so it
# could not be moved to fit the answer: run the paid sweep iff >= 10 seeds survive the
# ThreadSanitizer filter. That harvest returned 9 and the sweep was declined. Experiment
# 29 raised the MEASUREMENT to 10 by implementing a route the decline document itself
# named, and the bar is unchanged at 10. Both directions of this constant matter: it is
# also what stops a future operator fix from being read as licence to run a sweep the
# arithmetic below does not support.
REGISTERED_BAR = 10

# Measured: internal/verify/scarfree_coverage_test.go pins the site counts; the seed
# counts come from the compile + DATA-RACE filter (ScarFreeRaceKilledMutants), ~20 s.
#
# escape-defer is the deferred form of the escape operator, added in experiment 29. It
# is binned separately from escape rather than pooled because the two edit different
# things — the deferred form must convert `defer mu.Unlock()` into a plain call, which
# is what makes it the only operator here producing a -race-ONLY seed (below).
SITES = {"defer-wait": 8, "downgrade": 6, "escape": 1, "escape-defer": 1}
SEEDS = {"defer-wait": 4, "downgrade": 4, "escape": 1, "escape-defer": 1}
SEEDS_BY_PROBLEM = {
    "conc_parallel_map": 0, "conc_parallel_sum": 1, "conc_safe_counter": 0,
    "conc_parallel_filter": 1, "conc_fan_in_merge": 2, "conc_first_success": 1,
    "conc_parallel_histogram": 1, "conc_bounded_pipeline": 0, "hard_conc_rate_limiter": 2,
    "hard_conc_once_init": 2, "hard_conc_ordered_fanout": 0,
}

# 9 of the 10 seeds are ALSO refuted by the plain (no -race) test run. That is a fact
# about the ladder, not directly about the judge: it says the `-race` rung is not
# load-bearing for those seeds, since an ordinary run already refutes them. It does NOT
# by itself establish that a reader would notice — a deferred wg.Wait() with a balanced
# Add/Done is scar-free on the page whether or not the failure is deterministic — but it
# does mean those seeds test "judge vs a deterministic wrong answer" more than "judge vs
# an interleaving it must reason about", which is weaker than the arm was designed to be.
#
# Experiment 28 reported this at 9 of 9, i.e. NO seed needed the rung. The tenth seed
# (escape-defer on hard_conc_rate_limiter.Refill) is the first exception: undeferring the
# Unlock widens the window enough that the ordinary run passes and only ThreadSanitizer
# objects. So the arm now contains at least one seed of exactly the intended shape, which
# is a bigger change to what the sweep would measure than the count going 9 -> 10.
#
# Recorded, not filtered on: filtering one arm and not the other would make the two
# eta_fa rates incomparable, which is the whole reason raceKilledFrom is shared.
SEEDS_ALSO_PLAIN_REFUTED = 9


def main() -> None:
    print("=" * 78)
    print("SCAR-FREE RACE SEEDING — coverage on the 11-problem concurrency benchmark")
    print("=" * 78)
    print(f"  AST sites: {sum(SITES.values())}   usable seeds: {SCARFREE_SEEDS}"
          f"   (sync-deletion control: 0/{DELETION_SEEDS})")
    print("\n  per operator (sites -> seeds):")
    for op, label in (("defer-wait", "defer wg.Wait()          "),
                      ("downgrade", "RWMutex downgrade        "),
                      ("escape", "escape past Unlock       "),
                      ("escape-defer", "escape, undefer Unlock   ")):
        print(f"    {label} {SITES[op]} sites -> {SEEDS[op]} seeds")
    print("\n  The downgrade operator was FIRST MEASURED AT 0 seeds and reported as")
    print("  'structurally dead: no sync.RWMutex in the benchmark'. That was a bug in")
    print("  the operator (call sites rewritten, declaration left as sync.Mutex), not a")
    print("  property of the corpus. Co-mutating the declaration gives it 4 seeds — it")
    print("  is now co-equal with defer-wait, and the 'one operator carries the set'")
    print("  finding is RETRACTED.")
    print("\n  The escape operator was ALSO under-reaching: it skipped every")
    print("  `Lock(); defer Unlock()` site, which is the dominant Go idiom, so the one")
    print("  escape seed came from the rarer explicit-Unlock form. The deferred form is")
    print("  now implemented (experiment 29) and supplies the tenth seed. It needs a")
    print("  control-flow veto the plain form does not: undeferring an Unlock only")
    print("  unlocks on the fall-through path, so a `return` in the covered region")
    print("  DEADLOCKS instead of racing — the wrong defect class, and a visible lock")
    print("  imbalance besides, which is the deletion arm's territory.")
    print(f"\n  problems yielding >=1 seed: "
          f"{sum(1 for v in SEEDS_BY_PROBLEM.values() if v)}/{len(SEEDS_BY_PROBLEM)}")
    print(f"  seeds also refuted WITHOUT -race: {SEEDS_ALSO_PLAIN_REFUTED}"
          f"/{SCARFREE_SEEDS} (see SEEDS_ALSO_PLAIN_REFUTED — a caveat on seed quality,")
    print("  deliberately not a filter)")
    race_only = SCARFREE_SEEDS - SEEDS_ALSO_PLAIN_REFUTED
    print(f"  seeds refuted ONLY under -race: {race_only}"
          f" ({'none' if not race_only else 'escape-defer on hard_conc_rate_limiter'})")
    if race_only:
        print("    ^ experiment 28 had 0 of these, so its whole seed set tested the")
        print("      judge against deterministically-wrong programs. At least one seed")
        print("      now requires the interleaving, which is what the arm is for.")

    print("\n" + "=" * 78)
    print(f"CAN {SCARFREE_SEEDS} SEEDS ANSWER THE QUESTION?")
    print("=" * 78)
    crit = crit_value(SCARFREE_SEEDS, 0, DELETION_SEEDS)
    print(f"Fisher one-sided against the sync-deletion 0/{DELETION_SEEDS}:")
    print(f"\n  {'scar-free accepts':>18} {'rate':>7} {'p':>9}   verdict")
    for a in range(SCARFREE_SEEDS + 1):
        p = fisher_one_sided(a, SCARFREE_SEEDS - a, 0, DELETION_SEEDS)
        verdict = ("DECISIVE" if p < 0.01 else
                   "significant" if p < 0.05 else "cannot resolve")
        print(f"  {f'{a}/{SCARFREE_SEEDS}':>18} {a / SCARFREE_SEEDS:>7.3f} {p:>9.4f}"
              f"   {verdict}")

    null_bound = cp_upper(0, SCARFREE_SEEDS)
    print(f"\n  critical value: >={crit} of {SCARFREE_SEEDS}")
    print(f"  a null (0/{SCARFREE_SEEDS}) bounds scar-free eta_fa at <= {null_bound:.3f} (95%)")
    print("\n  power at the critical value, and the probability of at least one event")
    print("  (the existence-proof branch, which needs no p-value at all):")
    print(f"    {'true eta_fa':>12} {'power':>8} {'P(>=1 event)':>14}")
    for eta in (0.10, 0.20, 0.30, 0.40):
        print(f"    {eta:>12.2f} {binom_tail(crit, SCARFREE_SEEDS, eta):>8.3f}"
              f" {1 - (1 - eta) ** SCARFREE_SEEDS:>14.3f}")

    print("\n" + "=" * 78)
    print("THE DECISION")
    print("=" * 78)
    print(f"  Registered bar (set BEFORE experiment 28's harvest, so it could not be")
    print(f"  moved to fit the result): run the sweep iff >= {REGISTERED_BAR} seeds")
    print(f"  survive the DATA-RACE filter. Harvested now: {SCARFREE_SEEDS}.")
    verdict = "RUN" if SCARFREE_SEEDS >= REGISTERED_BAR else "DECLINE"
    print(f"  -> {verdict}")
    if SCARFREE_SEEDS >= REGISTERED_BAR:
        print(f"\n  The bar is MET, at the margin, and met the only legitimate way: by")
        print("  moving the measurement rather than the threshold. Experiment 28 declined")
        print(f"  at 9; the deferred-escape operator supplied the tenth seed, and the Go")
        print("  test pinned at 9 failed on the way in, which is how anyone noticed.")
        print("\n  Read the arithmetic above before funding it, though, because clearing a")
        print("  bar is not the same as being well-powered. What n=10 buys is an")
        print(f"  EXISTENCE PROOF with P={1 - 0.8 ** SCARFREE_SEEDS:.0%} under eta_fa=0.2, "
              "not a rate: a null")
        print(f"  bounds eta_fa only at <= {cp_upper(0, SCARFREE_SEEDS):.3f}. The case for "
              "spending is the")
        print("  existence branch, where a single scar-free false acceptance turns §3.1's")
        print("  mechanism from argued into demonstrated. The case against is that a null")
        print("  resolves nothing. That trade is the human's call, not this script's.")
        print("\n  SPEND IS NOT SELF-AUTHORIZING. Quote a real price against a past bill")
        print("  (results/README.md), not a per-token estimate, and get it approved.")
        print("\n  That happened: priced at ~$1.20 (the seeded path skips the cascade tier")
        print("  loop, so it is cheaper than the ~$3 the decline assumed), authorized")
        print("  explicitly, and RUN. See the experiment-30 block below for the result.")

    print("\n" + "=" * 78)
    print("WHAT THE SWEEP ACTUALLY RETURNED (experiment 30)")
    print("=" * 78)
    print(f"  scar-free   0/{SCARFREE_SEEDS_EXP30} false acceptances "
          f"(5 problems), all three strictness levels")
    print(f"  sync-delete 0/{DELETION_SEEDS_EXP30} false acceptances "
          f"(8 problems), all three strictness levels")
    p30 = fisher_one_sided(0, SCARFREE_SEEDS_EXP30, 0, DELETION_SEEDS_EXP30)
    print(f"\n  Fisher one-sided, 0/{SCARFREE_SEEDS_EXP30} vs "
          f"0/{DELETION_SEEDS_EXP30}: p = {p30:.4f}")
    print(f"  critical value was >={crit_value(SCARFREE_SEEDS_EXP30, 0, DELETION_SEEDS_EXP30)}"
          f" of {SCARFREE_SEEDS_EXP30}; it returned 0.")
    print(f"  scar-free null bound: eta_fa <= {cp_upper(0, SCARFREE_SEEDS_EXP30):.3f}")
    print(f"  deletion  null bound: eta_fa <= {cp_upper(0, DELETION_SEEDS_EXP30):.3f}")
    print("\n  The comparison the sweep exists to make asks whether the scar-free rate")
    print("  is ABOVE the deletion rate. Both are 0.000, so there is no gap to test and")
    print("  §3.1's reading-invisible mechanism stays ARGUED on its one live event.")
    print("  The surviving asymmetry is in BOUND TIGHTNESS, not observed rate: 9 seeds")
    print("  bound the scar-free class only at 0.283 while 27 bound the control at")
    print("  0.105. Overlapping intervals are not a mechanism — but the class §3.1 is")
    print("  about is the one we can least afford to sample, which is a fact about the")
    print("  operators' reach, not about the judge.")
    print("\n  Do NOT read the zeros as eta_fa = 0.")

    print("\n" + "=" * 78)
    print("WHAT WOULD MAKE IT BETTER-POWERED")
    print("=" * 78)
    print("  A larger denominator. At 0 events, and the critical value at each n:")
    for n in (6, 9, 10, 17, 23, 40, 60):
        c = crit_value(n, 0, DELETION_SEEDS)
        print(f"    n={n:<3} -> eta_fa <= {cp_upper(0, n):.3f}   crit >= {c}"
              f"   power at eta=0.3: {binom_tail(c, n, 0.30):.3f}")
    print("\n  Route (a) has now been MEASURED, for $0, and it does not help — which is")
    print("  worth more than the seed count, because it was the route both the decline")
    print("  document and this script called the cheapest and most promising:")
    print("  (a) Measure on the population the sweep actually mutates. ProfileSeeded")
    print("      draws a TIER-0 MODEL CANDIDATE and mutates the cluster winner — it")
    print("      never touches a reference, so the count above is a reference-only")
    print("      proxy, and model-authored concurrent code is where the study's one live")
    print("      scar-free false acceptance came from. Harvested offline from the")
    print("      retained sources of experiment 25: 13 sites -> 8 seeds NAIVELY, but the")
    print("      denominator is the point. 9 rows are only 7 unique programs")
    print("      (conc_safe_counter appears 3x byte-identically), so unique programs")
    print("      give 6, and restricting to bases that are themselves execution-correct")
    print("      gives 5 — ProfileSeeded does not check base correctness, so mutants of")
    print("      an already-wrong program would be counted as seeds. 5-8 depending on")
    print("      the denominator is NOT clearly above the reference-harvested count, so")
    print("      this route is closed as a way to raise n. All 7 unique bases are")
    print("      race-clean, which does confirm the operators cause the races.")
    print("  (b) Widen the corpus with multi-statement critical sections. Note this")
    print("      now revives only the ESCAPE operators; downgrade and deferred-escape")
    print("      are both alive. Caution: hand-authoring problems chosen to be mutable")
    print("      BY THESE OPERATORS tunes the benchmark to the instrument. Any such")
    print("      extension has to be written as a plausible Go exercise first and")
    print("      checked for operator coverage second, and the write-up must say which")
    print("      problems were added after the operators existed.")


if __name__ == "__main__":
    main()
