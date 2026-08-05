#!/usr/bin/env python3
"""Power arithmetic for the seeded scar-free race sweep (experiment 28).

    python3 results/scarfree_coverage.py

The scar-free race operators (internal/verify/mutation.go, PR #56) exist to seed the
one defect class §3.1 is actually about: racy code whose synchronization scaffolding is
intact, so a reading-only judge finds nothing missing. The sync-deletion operator cannot
produce that shape — it leaves a WaitGroup with no Wait — and scored 20/20 against the
judge precisely because imbalance is visible on the page.

Measured coverage on the 11-problem concurrency benchmark: **15 AST sites, 6 usable
seeds** (compile + `-race`-refuted). This script asks whether 6 seeds can answer the
question the sweep exists to answer, and the answer is no in the direction it would most
likely land. It costs nothing to run and it is why the paid sweep was declined.

No scipy: Clopper-Pearson by bisection on the exact tail sum, Fisher by enumerating the
hypergeometric tail. At n=6 a normal approximation is meaningless.
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


SCARFREE_SEEDS = 6
DELETION_SEEDS = 17
DELETION_PRIOR_N = 20  # results/race-seeded-2026-07-25.md: 20/20 caught, 0 false accepts

# Measured: internal/verify/scarfree_coverage_test.go pins the site counts; the seed
# counts come from the compile+race filter (ScarFreeRaceKilledMutants), ~11 min offline.
SITES = {"defer-wait": 8, "downgrade": 6, "escape": 1}
SEEDS_BY_PROBLEM = {
    "conc_parallel_map": 0, "conc_parallel_sum": 1, "conc_safe_counter": 1,
    "conc_parallel_filter": 1, "conc_fan_in_merge": 1, "conc_first_success": 1,
    "conc_parallel_histogram": 0, "conc_bounded_pipeline": 0,
    "hard_conc_rate_limiter": 0, "hard_conc_once_init": 1, "hard_conc_ordered_fanout": 0,
}


def main() -> None:
    print("=" * 76)
    print("SCAR-FREE RACE SEEDING — coverage on the 11-problem concurrency benchmark")
    print("=" * 76)
    print(f"  AST sites: {sum(SITES.values())}   usable seeds: {SCARFREE_SEEDS}"
          f"   (sync-deletion seeds: {DELETION_SEEDS})")
    print("\n  per operator (sites -> fate):")
    print(f"    defer wg.Wait()      {SITES['defer-wait']} sites -> 5 seeds  "
          "(the only productive operator)")
    print(f"    RWMutex downgrade    {SITES['downgrade']} sites -> 0 seeds  "
          "DEAD: no sync.RWMutex in the benchmark, every mutant fails to build")
    print(f"    escape past Unlock   {SITES['escape']} sites -> 1 seed")
    print(f"\n  problems yielding >=1 seed: {sum(1 for v in SEEDS_BY_PROBLEM.values() if v)}"
          f"/{len(SEEDS_BY_PROBLEM)}")

    print("\n" + "=" * 76)
    print("CAN 6 SEEDS ANSWER THE QUESTION?")
    print("=" * 76)
    print("The sweep compares scar-free eta_fa against the sync-deletion rate, which is")
    print(f"0 false acceptances of {DELETION_PRIOR_N} judged. Fisher one-sided, vs 0/{DELETION_SEEDS}:")
    print(f"\n  {'scar-free accepts':>18} {'rate':>7} {'p':>9}   verdict")
    for a in range(SCARFREE_SEEDS + 1):
        p = fisher_one_sided(a, SCARFREE_SEEDS - a, 0, DELETION_SEEDS)
        verdict = ("DECISIVE" if p < 0.01 else
                   "significant" if p < 0.05 else "CANNOT RESOLVE")
        print(f"  {f'{a}/{SCARFREE_SEEDS}':>18} {a / SCARFREE_SEEDS:>7.3f} {p:>9.4f}   {verdict}")

    null_bound = cp_upper(0, SCARFREE_SEEDS)
    print(f"\n  A null result (0/{SCARFREE_SEEDS}) bounds scar-free eta_fa only at "
          f"<= {null_bound:.3f} (95%).")
    print("  So the LIKELY outcome — the judge catching all six, as it caught 20/20")
    print("  scar-bearing races — would not distinguish 'scar-free races are caught'")
    print(f"  from 'six seeds cannot tell you anything below {null_bound:.0%}'.")
    print("  Needing >=3 of 6 to clear p<0.05 means only a LARGE effect is detectable,")
    print("  and the whole hypothesis is that this class is subtler than the visible one.")

    print("\n" + "=" * 76)
    print("WHAT WOULD MAKE IT WORTH RUNNING")
    print("=" * 76)
    print("  A denominator where a null is informative. At 0 events:")
    for n in (6, 17, 23, 40, 60):
        print(f"    0/{n:<3} -> eta_fa <= {cp_upper(0, n):.3f}")
    print("\n  Reaching ~23 seeds means adding concurrency problems with sync.RWMutex and")
    print("  with multi-statement critical sections, which revives the two dead operators.")
    print("  Caution, and it is the reason this was not simply done: hand-authoring")
    print("  problems chosen to be mutable BY THESE OPERATORS tunes the benchmark to the")
    print("  instrument. Any such extension has to be written as a plausible Go exercise")
    print("  first and checked for operator coverage second, and the write-up must say")
    print("  which problems were added after the operators existed.")


if __name__ == "__main__":
    main()
