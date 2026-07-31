#!/usr/bin/env python3
"""Diagnose WHY the completed n=64 pinned run (experiment 12) certifies alpha=0.05
only with thresholds [1,1] — i.e. why deployable-alpha and the cost win are, at the
2:1:1 fan-out, mutually exclusive.

Offline, free, re-derivable from the committed records. See
results/tension-diagnosis-2026-07-30.md for the narrative.

Usage:
    python3 results/analyze_tension.py results/go-specialist-211-pinned-n64.execution.json
"""
import json
import math
import sys
from collections import Counter


def wilson_lcb(k, n, z=1.645):
    """One-sided Wilson lower confidence bound — mirrors internal/cluster.wilsonLCB."""
    if n <= 0:
        return 0.0
    p = k / n
    z2 = z * z
    centre = p + z2 / (2 * n)
    spread = z * math.sqrt(p * (1 - p) / n + z2 / (4 * n * n))
    return max(0.0, min(1.0, (centre - spread) / (1 + z2 / n)))


def main(path):
    recs = json.load(open(path))
    clean = [r for r in recs
             if not r.get("oracle_unsound") and not r.get("contaminated")]
    print(f"records={len(recs)} clean(gated)={len(clean)}\n")

    # 1. The Wilson ceiling: the score a UNANIMOUS tier (k=n) can reach, by fan-out.
    #    It never reaches 1.0, so any threshold pinned at 1.0 is unclearable at any
    #    finite fan-out.
    print("Wilson LCB ceiling for a unanimous tier (k=n), by fan-out:")
    for n in (1, 2, 3, 5, 10, 20):
        print(f"  n={n:2d} -> {wilson_lcb(n, n):.4f}")
    print()

    # 2. tier0 score separation: correct vs truly-wrong answers.
    #    At 2 samples the score is quantised to {0, ~0.27 (1/2), ~0.425 (2/2)}.
    t0c, t0w = [], []
    for r in clean:
        t0 = r["tiers"][0]
        tc = t0.get("true_correct")
        if tc is None:
            continue
        (t0c if tc else t0w).append(round(t0["score"], 3))
    print(f"tier0 score | truly correct: {dict(Counter(t0c))}")
    print(f"tier0 score | truly wrong:   {dict(Counter(t0w))}\n")

    # 3. Acceptance-risk events: tier0 answers the ORACLE passed (correct=True) but
    #    that are truly wrong. These are what a score threshold must screen out.
    risk_events = [(r["id"], r["tiers"][0]["score"]) for r in clean
                   if r["tiers"][0].get("correct") is True
                   and r["tiers"][0].get("true_correct") is False]
    print(f"tier0 acceptance-risk events (oracle-pass, truly-wrong): {len(risk_events)}")
    for i, s in risk_events:
        print(f"  {i}: score={s:.3f}")

    # 4. The empirical-risk blocker: tier0 answers the oracle REFUTED (correct=False)
    #    but whose cluster score still reaches the unanimity ceiling — a confident
    #    wrong answer indistinguishable from a confident right one at this fan-out.
    blockers = [(r["id"], r["tiers"][0]["score"]) for r in clean
                if r["tiers"][0].get("correct") is False
                and r["tiers"][0]["score"] >= 0.42]
    print(f"\ntier0 empirical-risk blockers (oracle-wrong at unanimity ceiling): "
          f"{len(blockers)}")
    for i, s in blockers:
        print(f"  {i}: score={s:.3f}  <- forces tau0 > ceiling, i.e. tau0 = 1.0")

    print("\nConclusion: correct and confidently-wrong tier0 answers share the 2-sample")
    print("ceiling (0.425), so no threshold below it separates them; rejecting the")
    print("blocker forces tau0=1.0 (never-accept) -> full escalation -> cost inversion.")
    print("A higher tier0 fan-out helps ONLY if the blocker's error is flaky (samples")
    print("disagree, its score drops), not confident (samples repeat it, score holds).")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1
         else "results/go-specialist-211-pinned-n64.execution.json")
