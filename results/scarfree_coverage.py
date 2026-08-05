#!/usr/bin/env python3
"""Power arithmetic for the seeded scar-free race sweep (experiment 28).

    python3 results/scarfree_coverage.py

The scar-free race operators (internal/verify/mutation.go, PR #56) exist to seed the
one defect class §3.1 is actually about: racy code whose synchronization scaffolding is
intact, so a reading-only judge finds nothing missing. The sync-deletion operator cannot
produce that shape — it leaves a WaitGroup with no Wait — and scored 20/20 against the
judge precisely because imbalance is visible on the page.

Measured coverage on the 11-problem concurrency benchmark: **15 AST sites, 9 usable
seeds** (compile + ThreadSanitizer-confirmed race). This script asks whether that can
answer the question the sweep exists to answer, and the answer is not quite — one seed
short of the bar registered before the harvest ran. It costs nothing and it is the whole
basis on which the paid sweep was declined.

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
hypergeometric tail. At n=9 a normal approximation is meaningless.
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


SCARFREE_SEEDS = 9

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

# The bar was registered BEFORE the corrected harvest ran, precisely so it could not
# be moved to fit the answer: run the paid sweep iff >= 10 seeds survive the
# ThreadSanitizer filter. The harvest returned 9. One short is still short.
REGISTERED_BAR = 10

# Measured: internal/verify/scarfree_coverage_test.go pins the site counts; the seed
# counts come from the compile + DATA-RACE filter (ScarFreeRaceKilledMutants), ~18 s.
SITES = {"defer-wait": 8, "downgrade": 6, "escape": 1}
SEEDS = {"defer-wait": 4, "downgrade": 4, "escape": 1}
SEEDS_BY_PROBLEM = {
    "conc_parallel_map": 0, "conc_parallel_sum": 1, "conc_safe_counter": 0,
    "conc_parallel_filter": 1, "conc_fan_in_merge": 2, "conc_first_success": 1,
    "conc_parallel_histogram": 1, "conc_bounded_pipeline": 0, "hard_conc_rate_limiter": 1,
    "hard_conc_once_init": 2, "hard_conc_ordered_fanout": 0,
}

# Every one of the 9 seeds is ALSO refuted by the plain (no -race) test run. That is a
# fact about the ladder, not directly about the judge: it says the `-race` rung is not
# load-bearing for these particular seeds, since an ordinary run already refutes them.
# It does NOT by itself establish that a reader would notice — a deferred wg.Wait() with
# a balanced Add/Done is scar-free on the page whether or not the failure is
# deterministic — but it does mean these seeds test "judge vs a deterministic wrong
# answer" more than "judge vs an interleaving it must reason about", which is weaker
# than the arm was designed to be. Recorded, not filtered on: filtering one arm and not
# the other would make the two eta_fa rates incomparable, which is the whole reason
# raceKilledFrom is shared.
SEEDS_ALSO_PLAIN_REFUTED = 9


def main() -> None:
    print("=" * 78)
    print("SCAR-FREE RACE SEEDING — coverage on the 11-problem concurrency benchmark")
    print("=" * 78)
    print(f"  AST sites: {sum(SITES.values())}   usable seeds: {SCARFREE_SEEDS}"
          f"   (sync-deletion control: 0/{DELETION_SEEDS})")
    print("\n  per operator (sites -> seeds):")
    for op, label in (("defer-wait", "defer wg.Wait()     "),
                      ("downgrade", "RWMutex downgrade   "),
                      ("escape", "escape past Unlock  ")):
        print(f"    {label} {SITES[op]} sites -> {SEEDS[op]} seeds")
    print("\n  The downgrade operator was FIRST MEASURED AT 0 seeds and reported as")
    print("  'structurally dead: no sync.RWMutex in the benchmark'. That was a bug in")
    print("  the operator (call sites rewritten, declaration left as sync.Mutex), not a")
    print("  property of the corpus. Co-mutating the declaration gives it 4 seeds — it")
    print("  is now co-equal with defer-wait, and the 'one operator carries the set'")
    print("  finding is RETRACTED.")
    print(f"\n  problems yielding >=1 seed: "
          f"{sum(1 for v in SEEDS_BY_PROBLEM.values() if v)}/{len(SEEDS_BY_PROBLEM)}")
    print(f"  seeds also refuted WITHOUT -race: {SEEDS_ALSO_PLAIN_REFUTED}"
          f"/{SCARFREE_SEEDS} (see SEEDS_ALSO_PLAIN_REFUTED — a caveat on seed quality,")
    print("  deliberately not a filter)")

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
    print(f"  Registered bar (set BEFORE the harvest, so it could not be moved to fit")
    print(f"  the result): run the sweep iff >= {REGISTERED_BAR} seeds survive the")
    print(f"  DATA-RACE filter. Harvested: {SCARFREE_SEEDS}.")
    verdict = "RUN" if SCARFREE_SEEDS >= REGISTERED_BAR else "DECLINE"
    print(f"  -> {verdict}")
    if SCARFREE_SEEDS < REGISTERED_BAR:
        print(f"\n  One seed short. The honest reading is that this is CLOSE, not clear:")
        print(f"  at n={SCARFREE_SEEDS} an existence proof has "
              f"{1 - 0.8 ** SCARFREE_SEEDS:.0%} probability under eta_fa=0.2, which is")
        print("  not nothing. The bar is honoured anyway, because a bar moved after")
        print("  seeing the data is not a bar. What it would take to clear it is below.")

    print("\n" + "=" * 78)
    print("WHAT WOULD MAKE IT WORTH RUNNING")
    print("=" * 78)
    print("  A larger denominator. At 0 events, and the critical value at each n:")
    for n in (6, 9, 10, 17, 23, 40, 60):
        c = crit_value(n, 0, DELETION_SEEDS)
        print(f"    n={n:<3} -> eta_fa <= {cp_upper(0, n):.3f}   crit >= {c}"
              f"   power at eta=0.3: {binom_tail(c, n, 0.30):.3f}")
    print("\n  Two routes, and the cheaper one is NOT authoring problems:")
    print("  (a) Measure on the population the sweep actually mutates. ProfileSeeded")
    print("      draws a TIER-0 MODEL CANDIDATE and mutates the cluster winner — it")
    print("      never touches a reference. The 9 above is a reference-only proxy, and")
    print("      model-authored concurrent code is where the study's one live scar-free")
    print("      false acceptance came from. Harvesting from model draws is nearly free")
    print("      (one tier-0 sample per problem) and could clear the bar on its own.")
    print("  (b) Widen the corpus with multi-statement critical sections. Note this")
    print("      revives only the escape operator now; the downgrade one is alive.")
    print("      Caution: hand-authoring problems chosen to be mutable BY THESE")
    print("      OPERATORS tunes the benchmark to the instrument. Any such extension")
    print("      has to be written as a plausible Go exercise first and checked for")
    print("      operator coverage second, and the write-up must say which problems")
    print("      were added after the operators existed.")


if __name__ == "__main__":
    main()
