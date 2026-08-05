#!/usr/bin/env python3
"""Restate policy cost ratios NET OF the shared oracle, which no Record stores.

    python3 results/total_cost.py results/s55-fixed.records.execution.json

Every cost ratio in results/ and in the paper's §5.6 is a *routing* ratio: it compares
`tiers[].cost_usd`, which is what the tier calls were billed. The test-generation call
that produces the oracle is made once per problem, before any routing decision, and is
recorded nowhere — every `r.spec(...)` caller in internal/cascade passes a throwaway
`&Result{}`.

Because that term is identical under every routing policy, it **cancels exactly** in any
paired comparison between oracles on identical candidates. That is why the §5.5 results
are unaffected by the omission and why it survived 27 experiments without producing a
wrong comparative number. It does **not** cancel in a comparison between *policies*,
which is what a cost claim is, and there it dominates: measured at $0.0408/problem
against a frontier-tier routing cost of ~$0.00616/query, i.e. ~6.6x.

The collapse this computes is structural, not a property of one measurement: for a
shared cost S and frontier routing cost f, a routing ratio R becomes (S+f)/(S+f/R),
which tends to 1 as S/f grows, WHATEVER R is. So the script also prints the break-even
oracle price for a target total-cost ratio — the number that decides whether a cheaper
spec model is worth pursuing, and the reason the answer is "only if it is ~11x cheaper."

`-spec` overrides the oracle price. The default is the one live measurement
(pinned spec, ~2,700 output tokens = two Go test partitions at a frontier output rate),
and the total-cost ratios are sensitive to it, so it is a flag rather than a constant:
reconcile it against a provider bill before quoting a total-cost figure.
"""

import json
import pathlib
import sys

DEFAULT_SPEC_USD = 0.0408  # measured live, per problem; see results/README.md


def truth(t: dict) -> bool:
    """Execution ground truth. Mirrors TierObs.truth()."""
    return t["true_correct"] if t.get("true_correct") is not None else t["correct"]


def usable(recs: list[dict]) -> list[dict]:
    """Records surviving the calibration gates (invariants #3, #4). Timed-out records
    are kept — excluding them selects the sample on an outcome (invariant #8)."""
    return [r for r in recs
            if not r.get("oracle_unsound") and not r.get("contaminated")]


def route(rec: dict, taus: list[float]) -> tuple[bool, float]:
    """Replay routing under a threshold vector; the final tier has no threshold
    (invariant #6)."""
    cost = 0.0
    for i, t in enumerate(rec["tiers"]):
        cost += t["cost_usd"]
        if i >= len(taus) or t["score"] >= taus[i]:
            return truth(t), cost
    return truth(rec["tiers"][-1]), cost


def main() -> int:
    argv = sys.argv[1:]
    spec = DEFAULT_SPEC_USD
    if "-spec" in argv:
        i = argv.index("-spec")
        if i + 1 >= len(argv):
            sys.exit("-spec needs a USD-per-problem value")
        spec = float(argv[i + 1])
        del argv[i:i + 2]
    if len(argv) != 1:
        sys.exit(__doc__)
    p = pathlib.Path(argv[0])
    if not p.exists():
        sys.exit(f"missing {p}")
    recs = usable(json.loads(p.read_text()))
    n = len(recs)
    if not n:
        sys.exit("no usable records")

    frontier = sum(r["tiers"][-1]["cost_usd"] for r in recs) / n
    frisk = sum(1 for r in recs if not truth(r["tiers"][-1])) / n

    print("=" * 78)
    print(f"POLICY COST NET OF THE SHARED ORACLE — n={n}")
    print("=" * 78)
    print(f"  oracle (test generation, once per problem): ${spec:.4f}")
    print(f"  frontier-tier routing cost:                 ${frontier:.5f}/query")
    ratio = spec / frontier
    note = ("<- the oracle is the dominant term" if ratio >= 2
            else "<- comparable terms; routing ratios survive better here")
    print(f"  ratio S/f = {ratio:.1f}x  {note}")
    print()
    print(f"  {'policy':<30} {'routing':>9} {'+oracle':>9} {'total':>9} "
          f"{'route x':>8} {'TOTAL x':>8} {'risk':>7}")

    rows = [("always-frontier", None)]
    ntiers = len(recs[0]["tiers"])
    for taus in ([0.1] + [1.0] * (ntiers - 2), [1.0] * (ntiers - 1)):
        rows.append((f"cascade tau={taus}", taus))

    for label, taus in rows:
        if taus is None:
            cost, risk = frontier, frisk
        else:
            tot = 0.0
            wrong = 0
            for r in recs:
                ok, c = route(r, taus)
                tot += c
                if not ok:
                    wrong += 1
            cost, risk = tot / n, wrong / n
        print(f"  {label:<30} {cost:>9.5f} {spec:>9.4f} {spec + cost:>9.5f} "
              f"{frontier / cost:>7.2f}x {(spec + frontier) / (spec + cost):>7.3f}x "
              f"{risk:>7.4f}")

    # The omniscient bound of experiment 27, restated on total cost.
    ideal = 0.0
    for r in recs:
        ideal += (r["tiers"][0]["cost_usd"] if truth(r["tiers"][0])
                  else sum(t["cost_usd"] for t in r["tiers"]))
    ideal /= n
    print(f"  {'omniscient tier-0 gate':<30} {ideal:>9.5f} {spec:>9.4f} "
          f"{spec + ideal:>9.5f} {frontier / ideal:>7.2f}x "
          f"{(spec + frontier) / (spec + ideal):>7.3f}x {'—':>7}")

    print("\n" + "=" * 78)
    print("BREAK-EVEN ORACLE PRICE for a target TOTAL-cost ratio")
    print("=" * 78)
    print("Solve (S+f)/(S+c) = target for S, at the cheapest certifiable cascade cost c.")
    print("This is what a cheaper spec model would have to achieve to matter.")
    tot = 0.0
    for r in recs:
        tot += route(r, [0.1] + [1.0] * (ntiers - 2))[1]
    cheap = tot / n
    print(f"\n  cheapest cascade routing cost c = ${cheap:.5f}")
    print(f"  {'target total':>13} {'needs oracle <=':>17} {'i.e. cheaper by':>17}")
    for target in (1.25, 1.5, 2.0):
        s = (target * cheap - frontier) / (1 - target)
        if s <= 0:
            print(f"  {target:>12.2f}x {'impossible at any price':>17}")
        else:
            print(f"  {target:>12.2f}x {s:>17.5f} {spec / s:>16.0f}x")
    print("\n  Caveat: a cheaper spec model is not a free win. A weaker test author")
    print("  writes buggier suites, and experiment 19 measured generated-oracle errors")
    print("  as OVER-rejections — they cost escalations, not risk, so they are invisible")
    print("  to the certificate. The trade-off is spec cost vs OracleUnsound rate vs")
    print("  escalation rate, under invariant #3 (test author != code author) as a hard")
    print("  constraint.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
