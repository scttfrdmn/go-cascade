#!/usr/bin/env python3
"""Bound what ANY cheap-tier gate can buy, from the n=409 records, for $0.

    python3 results/fanout_ceiling.py results/s55-fixed.records.execution.json

Experiments 9, 10, 12a, 13, 14 and 17 all turned the same knob: the tier-0 sample
fan-out, across 5:2:1 / 1:1:1 / 2:1:1 / 5:1:1, six live draws at 5:1:1 alone. The
question they were chasing is whether a higher fan-out lets a threshold *below* the
unanimity ceiling admit cheap-tier answers, which is the only way the cascade is both
certifiable at a deployable alpha and cheaper than always-frontier.

Experiment 14 answered it as a theorem: fan-out buys separation against *flaky* wrong
answers and none against *confident* ones. Experiment 17 measured the consequence as a
frequency — 2 of 6 draws won — but at n~53, where experiment 26 later showed a single
observation moves the certificate. This script re-derives the whole question at n=409,
where it is a rate rather than a coin flip, and adds the bound the earlier experiments
did not compute.

What it reports, in order:

  * The tier-0 score distribution. The point of printing it is that the gate has only
    as many reachable settings as there are distinct scores — three, at 2:1:1 — so the
    "threshold sweep" is not a continuum and reading a grid of 121 values suggests a
    tuning freedom that does not exist.
  * The confident-wrong rate: unanimous at the fan-out's Wilson ceiling (invariant #9)
    and refuted by execution. This is the class experiment 14 proved fan-out cannot
    touch. At n=409 it is a rate with a real denominator; at n~53 it was 0/1/2 events.
  * Whether that rate PREDICTS experiment 17's 2-of-6. If it does, the frequency was
    never a property of fan-out — it was small-sample sampling of a fixed rate, and no
    fan-out setting would have changed it.
  * **The omniscient bound.** A gate that accepts exactly the cheap-tier answers
    execution says are correct — unattainable by construction, since knowing that
    requires the answer. Every fan-out, every statistic, and every threshold is bounded
    by it. This is the number that decides whether more fan-out experiments are worth
    running, and no amount of live sampling produces it: it comes from having profiled
    every tier on every problem.

Ground truth is `true_correct` falling back to `correct`, mirroring `TierObs.truth()`.
Cost is recorded tier cost, which EXCLUDES the shared spec/oracle call (~74-91% of real
spend, in no Record) — so these are ratios between policies, never a bill.
"""

import collections
import json
import math
import pathlib
import sys


def truth(t: dict) -> bool:
    """Execution ground truth for a tier observation. Mirrors TierObs.truth()."""
    return t["true_correct"] if t.get("true_correct") is not None else t["correct"]


def usable(recs: list[dict]) -> list[dict]:
    """Records surviving the calibration gates (invariants #3, #4).

    A timed-out record is NOT dropped — excluding it would select the sample on an
    outcome (invariant #8).
    """
    return [r for r in recs
            if not r.get("oracle_unsound") and not r.get("contaminated")]


def route(rec: dict, taus: list[float]) -> tuple[int, bool, float]:
    """Replay routing under a threshold vector. The final tier has no threshold
    (invariant #6), so the walk always terminates."""
    cost = 0.0
    for i, t in enumerate(rec["tiers"]):
        cost += t["cost_usd"]
        if i >= len(taus) or t["score"] >= taus[i]:
            return i, truth(t), cost
    return len(rec["tiers"]) - 1, truth(rec["tiers"][-1]), cost


def clopper_pearson(k: int, n: int, conf: float = 0.95) -> tuple[float, float]:
    """Exact binomial CI by bisection on the tail sums. No scipy dependency, and no
    normal approximation — at n=6 the approximation is meaningless."""
    a = (1 - conf) / 2

    def cdf(kk: int, pp: float) -> float:
        return sum(math.comb(n, i) * pp ** i * (1 - pp) ** (n - i) for i in range(kk + 1))

    def bisect(f, target: float) -> float:
        lo, hi = 0.0, 1.0
        for _ in range(200):
            mid = (lo + hi) / 2
            if f(mid) > target:
                hi = mid
            else:
                lo = mid
        return (lo + hi) / 2

    low = 0.0 if k == 0 else bisect(lambda p: 1 - cdf(k - 1, p), a)
    high = 1.0 if k == n else bisect(lambda p: -cdf(k, p), -a)
    return low, high


def main() -> int:
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    p = pathlib.Path(sys.argv[1])
    if not p.exists():
        sys.exit(f"missing {p}")
    recs = usable(json.loads(p.read_text()))
    n = len(recs)
    if not n:
        sys.exit("no usable records")

    t0 = [r["tiers"][0] for r in recs]
    ceiling = max(t["score"] for t in t0)
    tier_names = [t["tier"] for t in recs[0]["tiers"]]

    print("=" * 78)
    print(f"TIER-0 SCORE DISTRIBUTION — n={n} usable, tiers {tier_names}")
    print("=" * 78)
    print("The gate has exactly as many reachable settings as there are distinct")
    print("scores. A grid of 121 thresholds over three values is not a continuum.")
    counts = collections.Counter(round(t["score"], 4) for t in t0)
    print(f"\n  {'score':>8} {'n':>5} {'wrong':>6} {'wrong-rate':>11}")
    for s in sorted(counts):
        sub = [t for t in t0 if round(t["score"], 4) == s]
        w = sum(1 for t in sub if not truth(t))
        print(f"  {s:>8} {len(sub):>5} {w:>6} {w / len(sub):>11.4f}")
    acc = sum(truth(t) for t in t0)
    print(f"\n  tier-0 accuracy {acc}/{n} = {acc / n:.4f}")
    print(f"  Wilson unanimity ceiling at this fan-out: {ceiling:.4f} (invariant #9)")

    # ---- the class fan-out provably cannot separate -------------------------------
    print("\n" + "=" * 78)
    print("CONFIDENT-WRONG RATE — the class experiment 14 proved fan-out cannot touch")
    print("=" * 78)
    at_ceiling = [r for r in recs if r["tiers"][0]["score"] >= ceiling - 1e-9]
    cw = [r for r in at_ceiling if not truth(r["tiers"][0])]
    cr = [r for r in at_ceiling if truth(r["tiers"][0])]
    rate = len(cw) / n
    print(f"\n  at the ceiling: {len(cr)} correct, {len(cw)} WRONG, all at score "
          f"{ceiling:.6f}")
    print(f"  confident-wrong rate p = {len(cw)}/{n} = {rate:.4f}")
    print("\n  Any threshold that rejects the wrong ones also rejects every correct")
    print("  one: they are numerically identical. That is the headroom theorem, now")
    print("  measured at this n rather than inferred from a handful of events.")
    if cw:
        print(f"\n  ids: {' '.join(sorted(r['id'] for r in cw))}")

    # ---- does the rate explain experiment 17's frequency? -------------------------
    print("\n" + "=" * 78)
    print("DOES THIS RATE EXPLAIN EXPERIMENT 17's 2-of-6?")
    print("=" * 78)
    print("Experiment 17's rule was exact across six 5:1:1 draws: zero confident-wrong")
    print("in the clean set => tau0 < 1 => cost win; one or more => tau0 = 1 => no win.")
    print("So P(win) is P(a clean set of size m contains none), and if the records'")
    print("rate is the underlying one, the fan-out setting never entered the question.")
    print(f"\n  {'m':>6} {'P(zero)':>9} {'E[count]':>9}")
    for m in (47, 53, 64, 100, 200, n):
        print(f"  {m:>6} {(1 - rate) ** m:>9.4f} {rate * m:>9.2f}")
    low, high = clopper_pearson(2, 6)
    pred = (1 - rate) ** 53
    inside = low <= pred <= high
    print(f"\n  experiment 17 observed 2/6 = 0.333 at m~53")
    print(f"  exact (Clopper-Pearson) 95% CI: [{low:.3f}, {high:.3f}]")
    print(f"  this rate predicts P(zero in 53) = {pred:.3f} -> "
          f"{'INSIDE' if inside else 'OUTSIDE'} the CI")
    if inside:
        print("\n  => experiment 17's frequency is consistent with sampling a FIXED rate.")
        print("     The 2-of-6 was small-n noise around a constant, not evidence about")
        print("     fan-out. Six more draws would re-estimate p, not move it.")

    # ---- the bound that decides whether to keep going -----------------------------
    print("\n" + "=" * 78)
    print("THE OMNISCIENT BOUND — what NO cheap-tier gate can beat")
    print("=" * 78)
    print("A gate that accepts exactly the cheap answers execution says are correct.")
    print("Unattainable by construction (knowing that requires the answer), so it")
    print("upper-bounds every fan-out, every statistic and every threshold vector.")
    ideal_cost = 0.0
    ideal_wrong = 0
    for r in recs:
        if truth(r["tiers"][0]):
            ideal_cost += r["tiers"][0]["cost_usd"]
        else:
            ideal_cost += sum(t["cost_usd"] for t in r["tiers"])
            if not truth(r["tiers"][-1]):
                ideal_wrong += 1
    # always-frontier pays ONLY the frontier tier — it never calls the others. The
    # cost of walking every tier is the tau=[1,1] CASCADE, which is a different and
    # strictly larger policy; conflating them inflated this bound from 1.83x to 2.99x
    # in a first version of this script. `calibrate -baselines` is the cross-check.
    frontier_cost = sum(r["tiers"][-1]["cost_usd"] for r in recs) / n
    frontier_risk = sum(1 for r in recs if not truth(r["tiers"][-1])) / n
    print(f"\n  {'policy':<34} {'$/query':>10} {'risk':>8} {'vs frontier':>12}")
    print(f"  {'omniscient tier-0 gate':<34} {ideal_cost / n:>10.5f} "
          f"{ideal_wrong / n:>8.4f} {frontier_cost / (ideal_cost / n):>11.2f}x")
    print(f"  {'always-frontier':<34} {frontier_cost:>10.5f} "
          f"{frontier_risk:>8.4f} {'1.00x':>12}")
    print(f"\n  Best possible speedup from ANY cheap-tier gate: "
          f"{frontier_cost / (ideal_cost / n):.2f}x")
    print("  Cost is recorded tier cost only. The shared spec/oracle call is in no")
    print("  Record and is ~74-91% of real spend, so these are policy ratios, never")
    print("  a bill.")

    # ---- realizable policies, and where their risk comes from --------------------
    print("\n" + "=" * 78)
    print("REALIZABLE POLICIES, and the tier each unit of risk is incurred at")
    print("=" * 78)
    print("Attribute risk per accepting tier before crediting or blaming tier 0: the")
    print("final tier has no threshold (invariant #6), so frontier misses are risk no")
    print("threshold vector can route around.")
    for taus in ([0.1, 1.0], [1.0, 1.0]):
        by = collections.Counter()
        wrong = collections.Counter()
        cost = 0.0
        for r in recs:
            i, ok, c = route(r, taus)
            cost += c
            by[i] += 1
            if not ok:
                wrong[i] += 1
        tot = sum(wrong.values())
        print(f"\n  tau={taus}: ${cost / n:.5f}/query, risk {tot / n:.4f} "
              f"({frontier_cost / (cost / n):.2f}x vs frontier)")
        for i in sorted(by):
            print(f"     tier {i} ({tier_names[i]:6s}): accepted {by[i]:4d}, "
                  f"wrong {wrong[i]:3d}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
