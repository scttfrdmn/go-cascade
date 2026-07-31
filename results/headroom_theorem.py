#!/usr/bin/env python3
"""The fan-out headroom theorem — why a higher cheap-tier fan-out helps against a
FLAKY wrong answer but not a CONFIDENT one, derived from the routing rule alone.

This is the draw-independent half of closing experiment 13's confound
(results/fanout-511-2026-07-30.md). It is NOT a model measurement and NOT a mock
number: it is an exact-binomial property of `internal/cluster.Score` (the Wilson
lower-confidence-bound routing statistic). No Bedrock, no candidates, no spend.

Model. A tier draws N independent samples. An answer that the model reproduces with
per-sample probability p forms a behavioural cluster of size k ~ Binomial(N, p)
(cluster.Behaviour keys on the test-outcome vector, so identical answers cluster).
The router accepts the tier iff the largest verified cluster clears the threshold,
i.e. wilsonLCB(k, N) >= tau. So P[accept] = P[Binom(N, p) >= k*(N, tau)], where
k*(N, tau) is the least k whose Wilson LCB reaches tau.

Claim. Let p_c be the per-sample probability of the *correct* answer and p_w that of
a *wrong* answer. The best achievable discrimination — max over thresholds of
(P[accept | p_c] - P[accept | p_w]) — WIDENS with N when p_w < p_c (a flaky wrong
answer) and stays 0 for all N when p_w = p_c (a confident wrong answer the model
reproduces as reliably as the right one). Fan-out buys headroom against flakiness,
never against confident error. This is exactly the dichotomy the tension diagnosis
asserted, now proven from the scoring rule.
"""
import math
from math import comb


def wilson_lcb(k, n, z=1.645):
    """Mirrors internal/cluster.wilsonLCB."""
    if n <= 0:
        return 0.0
    p = k / n
    z2 = z * z
    centre = p + z2 / (2 * n)
    spread = z * math.sqrt(p * (1 - p) / n + z2 / (4 * n * n))
    return max(0.0, min(1.0, (centre - spread) / (1 + z2 / n)))


def binom_ge(n, p, k):
    """P[Binomial(n, p) >= k], computed exactly."""
    return sum(comb(n, j) * p**j * (1 - p)**(n - j) for j in range(k, n + 1))


def best_discrimination(n, p_correct, p_wrong):
    """Over every threshold achievable at fan-out n (one per cluster size k=1..n),
    the largest gap between accepting a p_correct answer and a p_wrong one."""
    best = None
    for k in range(1, n + 1):
        gap = binom_ge(n, p_correct, k) - binom_ge(n, p_wrong, k)
        if best is None or gap > best["gap"]:
            best = {"gap": gap, "k": k, "tau": wilson_lcb(k, n),
                    "accept_correct": binom_ge(n, p_correct, k),
                    "accept_wrong": binom_ge(n, p_wrong, k)}
    return best


def table(p_correct, p_wrong, label, fanouts=(1, 2, 3, 5, 8, 10)):
    print(f"\n{label}  (correct p={p_correct}, wrong p={p_wrong})")
    print(f"  {'N':>3} {'best τ':>8} {'accept correct':>15} "
          f"{'accept wrong':>13} {'gap':>7}")
    for n in fanouts:
        b = best_discrimination(n, p_correct, p_wrong)
        print(f"  {n:>3} {b['tau']:>8.3f} {b['accept_correct']:>15.3f} "
              f"{b['accept_wrong']:>13.3f} {b['gap']:>7.3f}")


if __name__ == "__main__":
    print("Fan-out headroom theorem (exact binomial; a property of the Wilson score,")
    print("not a model measurement).")
    table(0.9, 0.5, "FLAKY wrong answer — gap WIDENS with N (fan-out helps)")
    table(0.9, 0.9, "CONFIDENT wrong answer — gap is 0 for all N (fan-out cannot help)")
    print("\nConclusion: raising the cheap-tier fan-out buys discrimination headroom")
    print("against flaky wrong answers and NONE against confident ones. This is why")
    print("5:1:1 could certify alpha=0.05 with cheap acceptance where 2:1:1 could not —")
    print("and why it works only when the tier's residual errors are flaky, which is an")
    print("empirical question the repeat-draw runs address, not this theorem.")
