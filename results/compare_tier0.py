#!/usr/bin/env python3
"""Compare two cascade arms that differ ONLY in tier 0's model, on one denominator.

    python3 results/compare_tier0.py \
        results/go-specialist-211-pinned-n64.execution.json \
        results/qwen-coder-211.records.execution.json

Why this is not `analyze_s55.py`: that script pairs two *oracles* over one candidate
stream, where `ProfilePaired` guarantees both arms see identical candidates and the same
exclusions. Here the arms are two separate live runs, so nothing is shared — not the
candidates, not the generated tests, and *not the exclusion set*. That last one is the
trap, and it is the reason this file exists rather than a handful of shell one-liners.

**The `-refs` gate excludes a different set of problems in each arm.** Oracle soundness
is a property of the generated test suite, and the suite is regenerated per run; on the
6-problem smoke slice the two arms' unsound ids were *disjoint* (`seq_longest_run` for
Qwen, `str_anagram` for Maverick). So the natural report — arm A's escalation rate over
its own usable n against arm B's over its own — compares two different problem sets and
attributes the difference to the model. That is the defect
`TestSelfConsistencySummaryPairsOnOneDenominator` exists to prevent in the arm-(e)
report: two individually-correct rates over different denominators read as a finding.

Hence every comparative figure below is computed over `both`, the intersection of the
two arms' usable records. The per-arm-n figures are printed too, but labelled
incomparable — they are there to show how much the denominators move, which is itself a
hazard the scope doc predicted ("a differently-flawed tier 0 moves n; quote the
denominator").

What it reports:

  * The denominators, and the exclusion sets, explicitly and first.
  * Tier-0 accuracy over `both`. This is the quantity a coder specialist is supposed to
    move, and it is measured against execution ground truth (`true_correct`), never
    against the score.
  * Escalation rate under each threshold vector. This is the *predicted* win: tier 0 is
    only ~4% of spend, so a cheaper tier-0 line cannot pay for itself — fewer
    escalations must. Escalation is derived from the recorded scores and a threshold
    vector, not read from a field, because the records profile every tier on every
    problem (that is what makes the whole sweep replayable offline).
  * Mean cost per query under each threshold vector, over `both`.
  * McNemar's exact test on tier-0 correctness. With n≈50 and a per-problem pairing,
    the discordant pairs are the entire signal; a difference in marginal rates over a
    common denominator can still be noise, and the paired test is what says whether it
    is. Reported as an exact binomial two-sided p, no normal approximation, matching
    the arm-(e) write-up's convention.

Ground truth is `true_correct` falling back to `correct`, mirroring `TierObs.truth()`
in internal/calibrate/ltt.go.

With `-pair-out <stem>` it also writes `<stem>.A.json` / `<stem>.B.json`, each arm
restricted to the paired denominator, so that

    go-cascade calibrate -from-records <stem>.A.json -alpha X -delta 0.10 -baselines

certifies each arm through the **real** LTT implementation on one denominator. The
alternative — reimplementing Hoeffding-Bentkus and the fixed-sequence ordering here —
would put a second, untested copy of a load-bearing statistical claim into a throwaway
analysis script (invariant #7 lives in `Calibrate`, and its grid ordering must stay
data-independent). Escalation and cost above are safe to recompute here because they
are arithmetic over recorded scores; a *certificate* is not.
"""

import json
import math
import pathlib
import sys
from itertools import product


def truth(t: dict) -> bool:
    """Execution ground truth for a tier observation. Mirrors TierObs.truth()."""
    return t["true_correct"] if t.get("true_correct") is not None else t["correct"]


def load(path: str) -> list[dict]:
    p = pathlib.Path(path)
    if not p.exists():
        sys.exit(f"missing {p}")
    return json.loads(p.read_text())


def usable(recs: list[dict]) -> dict:
    """Records that survive the calibration gates, keyed by id.

    Mirrors the exclusions `Calibrate` applies: an oracle-unsound record's labels are
    noise (invariant #4) and a contaminated one violates invariant #3. A timed-out
    record is NOT excluded here — that would select the sample on an outcome
    (invariant #8); the flag is forensic and is tallied separately below.
    """
    return {r["id"]: r for r in recs
            if not r.get("oracle_unsound") and not r.get("contaminated")}


def route(rec: dict, taus: list[float]) -> tuple[int, bool, float]:
    """Replay routing for one record under a threshold vector.

    Returns (tier index accepted at, correct, cumulative cost). The final tier has no
    threshold — that is invariant #6, by construction, not an oversight — so the walk
    always terminates there.
    """
    cost = 0.0
    tiers = rec["tiers"]
    for i, t in enumerate(tiers):
        cost += t["cost_usd"]
        if i >= len(taus):          # final tier: accept unconditionally
            return i, truth(t), cost
        if t["score"] >= taus[i]:
            return i, truth(t), cost
    last = len(tiers) - 1
    return last, truth(tiers[last]), cost


def mcnemar_exact(b: int, c: int) -> float:
    """Two-sided exact binomial p for b vs c discordant pairs (Sign test at p=0.5)."""
    n = b + c
    if n == 0:
        return 1.0
    k = min(b, c)
    tail = sum(math.comb(n, i) for i in range(k + 1)) / (2 ** n)
    return min(1.0, 2 * tail)


def pct(k: int, n: int) -> str:
    return f"{k}/{n} = {k / n:.4f}" if n else f"{k}/0 = n/a"


def main() -> int:
    argv = sys.argv[1:]
    pair_out = None
    if "-pair-out" in argv:
        i = argv.index("-pair-out")
        if i + 1 >= len(argv):
            sys.exit("-pair-out needs a path stem")
        pair_out = argv[i + 1]
        del argv[i:i + 2]
    if len(argv) != 2:
        sys.exit(__doc__)
    a_path, b_path = argv[0], argv[1]
    a_all, b_all = load(a_path), load(b_path)
    a, b = usable(a_all), usable(b_all)
    a_name = pathlib.Path(a_path).name
    b_name = pathlib.Path(b_path).name

    both = sorted(set(a) & set(b))

    print("=" * 78)
    print("DENOMINATORS — read these before any rate below")
    print("=" * 78)
    for nm, all_, us in ((a_name, a_all, a), (b_name, b_all, b)):
        uns = sorted(r["id"] for r in all_ if r.get("oracle_unsound"))
        con = sorted(r["id"] for r in all_ if r.get("contaminated"))
        to = sum(1 for r in all_ if any(t.get("timed_out") for t in r["tiers"]))
        print(f"\n{nm}")
        print(f"  records {len(all_)}  usable {len(us)}")
        print(f"  oracle-unsound {len(uns)}: {' '.join(uns) if uns else '(none)'}")
        if con:
            print(f"  contaminated {len(con)}: {' '.join(con)}")
        print(f"  timed-out records {to} (KEPT — forensic only, invariant #8)")

    only_a = sorted(set(a) - set(b))
    only_b = sorted(set(b) - set(a))
    print(f"\nPAIRED DENOMINATOR: {len(both)} problems usable in BOTH arms.")
    print(f"  usable only in {a_name}: {len(only_a)} {' '.join(only_a)}")
    print(f"  usable only in {b_name}: {len(only_b)} {' '.join(only_b)}")
    print("  Every comparative figure below is over the paired denominator. The gate")
    print("  excludes a different set per arm, so per-arm-n rates are NOT comparable.")

    # ---- tier-0 accuracy, the quantity a coder specialist should move -------------
    print("\n" + "=" * 78)
    print(f"TIER-0 ACCURACY (execution ground truth), paired over {len(both)}")
    print("=" * 78)
    a_ok = sum(truth(a[i]["tiers"][0]) for i in both)
    b_ok = sum(truth(b[i]["tiers"][0]) for i in both)
    print(f"  {a_name:52s} {pct(a_ok, len(both))}")
    print(f"  {b_name:52s} {pct(b_ok, len(both))}")

    disc_a = [i for i in both if truth(a[i]["tiers"][0]) and not truth(b[i]["tiers"][0])]
    disc_b = [i for i in both if truth(b[i]["tiers"][0]) and not truth(a[i]["tiers"][0])]
    p = mcnemar_exact(len(disc_a), len(disc_b))
    print(f"\n  discordant: {len(disc_a)} where A only, {len(disc_b)} where B only")
    print(f"  McNemar exact two-sided p = {p:.4g}")
    print("  (agreement is uninformative — the discordant pairs ARE the comparison)")
    if disc_a:
        print(f"    A only: {' '.join(disc_a)}")
    if disc_b:
        print(f"    B only: {' '.join(disc_b)}")

    # ---- escalation and cost, the predicted win ----------------------------------
    print("\n" + "=" * 78)
    print("ESCALATION RATE AND COST/QUERY by threshold vector, paired")
    print("=" * 78)
    print("Tier 0 is ~4% of recorded spend, so a cheaper tier-0 line cannot pay for")
    print("itself; the win must be fewer escalations. Both arms replay from records.")
    ntiers = len(next(iter(a.values()))["tiers"])
    grid = [list(v) for v in product([0.1, 0.25, 0.4, 1.0], repeat=ntiers - 1)]
    hdr = f"\n  {'tau':16s} {'arm':10s} {'accept@0':>9s} {'escalated':>10s} {'risk':>7s} {'$/query':>10s}"
    print(hdr)
    for taus in grid:
        for nm, arm in (("A", a), ("B", b)):
            acc0 = esc = wrong = 0
            cost = 0.0
            for i in both:
                tier, ok, c = route(arm[i], taus)
                cost += c
                if tier == 0:
                    acc0 += 1
                else:
                    esc += 1
                if not ok:
                    wrong += 1
            print(f"  {str(taus):16s} {nm:10s} {acc0:9d} {esc:10d} "
                  f"{wrong / len(both):7.4f} {cost / len(both):10.5f}")

    # ---- confident-wrong tier-0 answers: the actual mechanism of the cost win ----
    print("\n" + "=" * 78)
    print("CONFIDENT-WRONG TIER-0 ANSWERS, paired")
    print("=" * 78)
    print("Experiment 17 established across six draws that the deployable-alpha cost")
    print("win happens **iff** the clean calibration set contains zero confident-wrong")
    print("tier-0 answers — a unanimous cluster (score at the fan-out ceiling) on a")
    print("program execution says is wrong. Fan-out provably buys discrimination")
    print("against *flaky* cheap-tier errors and NONE against confident ones")
    print("(experiment 14's headroom theorem), so a threshold cannot separate them and")
    print("the certificate collapses to [1,1].")
    print()
    print("Read this before crediting any cost win to the model: in BOTH observed wins")
    print("the confident-wrong answers existed and were merely oracle-unsound-EXCLUDED,")
    print("so the win rode a coincidence between two independent error processes. A new")
    print("tier-0 model perturbs both, and the exclusion set is not the model's doing.")
    ceiling = max((t["score"] for arm in (a, b) for r in arm.values()
                   for t in (r["tiers"][0],)), default=0.0)
    print(f"\n  highest tier-0 score observed in either arm: {ceiling:.4f}")
    print("  (the unanimous Wilson ceiling for the configured fan-out; invariant #9)")
    for nm, arm, all_ in ((a_name, a, a_all), (b_name, b, b_all)):
        conf = [i for i in both
                if arm[i]["tiers"][0]["score"] >= ceiling - 1e-9
                and not truth(arm[i]["tiers"][0])]
        # The same class among the EXCLUDED records — the coincidence to check.
        excl = {r["id"]: r for r in all_ if r.get("oracle_unsound")}
        conf_excl = [i for i, r in excl.items()
                     if r["tiers"][0]["score"] >= ceiling - 1e-9
                     and not truth(r["tiers"][0])]
        print(f"\n  {nm}")
        print(f"    confident-wrong in the paired clean set: {len(conf)} "
              f"{' '.join(conf) if conf else '(none -> a cost win is possible)'}")
        print(f"    confident-wrong among oracle-unsound EXCLUSIONS: {len(conf_excl)} "
              f"{' '.join(sorted(conf_excl))}")
        if not conf and conf_excl:
            print("    ^ the experiment-17 coincidence: the class exists but is excluded,")
            print("      so a win here is NOT evidence of cheap-tier robustness.")

    # ---- per-tier spend, so the "cheaper per sample" claim is checkable ----------
    print("\n" + "=" * 78)
    print(f"RECORDED COST PER PROBLEM BY TIER, paired over {len(both)}")
    print("=" * 78)
    print("  Recorded tier cost EXCLUDES the spec/oracle call — no Record stores it,")
    print("  and it is ~91% of real spend. Do not read these as the bill.")
    print(f"\n  {'tier':10s} {'A $/problem':>14s} {'B $/problem':>14s} {'ratio A/B':>11s}")
    for t in range(ntiers):
        ca = sum(a[i]["tiers"][t]["cost_usd"] for i in both) / len(both)
        cb = sum(b[i]["tiers"][t]["cost_usd"] for i in both) / len(both)
        name = a[both[0]]["tiers"][t]["tier"]
        ratio = f"{ca / cb:.2f}x" if cb else "n/a"
        print(f"  {name:10s} {ca:14.6f} {cb:14.6f} {ratio:>11s}")

    # ---- the per-arm figures, printed but labelled ------------------------------
    print("\n" + "=" * 78)
    print("PER-ARM-n FIGURES — NOT comparable with each other")
    print("=" * 78)
    print("  Shown only so the denominator drift is visible. Each is correct over its")
    print("  own n; reading one against the other is the defect this script prevents.")
    for nm, arm in ((a_name, a), (b_name, b)):
        ids = sorted(arm)
        ok = sum(truth(arm[i]["tiers"][0]) for i in ids)
        print(f"  {nm:52s} tier-0 acc {pct(ok, len(ids))}")

    # ---- the paired subsets, for certification by the real LTT ------------------
    if pair_out:
        for suffix, arm in (("A", a), ("B", b)):
            p = pathlib.Path(f"{pair_out}.{suffix}.json")
            p.write_text(json.dumps([arm[i] for i in both], indent=1))
            print(f"\nwrote {p} ({len(both)} paired records)")
        print("  Certify each through the real implementation, one denominator:")
        print(f"    go-cascade calibrate -from-records {pair_out}.A.json "
              f"-alpha A -delta 0.10 -baselines -o /tmp/a.json")
        print(f"    go-cascade calibrate -from-records {pair_out}.B.json "
              f"-alpha A -delta 0.10 -baselines -o /tmp/b.json")
    return 0


if __name__ == "__main__":
    sys.exit(main())
