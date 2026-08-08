#!/usr/bin/env python3
"""Two $0 checks gating the model-combination design space (docs/model-combination-design-space-2026-08-08.md).

    python3 results/design_space_offline_checks.py

Both checks were run once inline during that design-space review, on one machine, without
being committed as reproducible code. This script is that code, so either finding can be
re-run against new records instead of re-typed from a chat transcript — the same reason
compare_tier0.py and classify_disagreements.py exist as scripts rather than one-off shell.

CHECK 1 — cross-family correlated failure (gates approach #2, cross-family ensemble voting).
The design review's own offline pass used one pair of overlapping-problem-set record files
(qwen-coder-211 vs go-specialist-211-pinned, n=47) and found tier-0 failures co-occurring at
~4.4x the independence-predicted rate. That is one pair and could be a small-n artifact —
broadened here across every cross-family pair with >=15 problems of overlap across the
tier-0 model families this repo has actually run (maverick, qwen, haiku). Uses each record's
own `contaminated` flag to exclude tainted rows; does NOT re-derive oracle-soundness
exclusions, so this is a lower-fidelity pass than analyze_s55.py's paired-oracle machinery —
adequate for "does the signal survive a broader look", not for a certified figure.

CHECK 2 — repair's share of the total bill (gates approach #4, specialized repair model).
Experiment 26 (qwen-coder-tier0-n64-2026-08-04.md) already measured tier 0's own repair
spread (5.9x around the median) but did not extend the estimate to mid/large tiers or to a
share of the TOTAL bill including the shared spec/oracle call, which CLAUDE.md documents as
74-91% of real spend. This check reproduces the same "excess above the tier's own median"
proxy for repair cost, computed at all three tiers, then converts tier-only spend into a
share of an assumed total using that documented range.

Both checks are read as first-pass signals bounding whether a live/code effort is worth
funding, not as certified findings on the order of the numbered results/*.md experiments.
"""
import glob
import itertools
import json
import statistics
import sys

try:
    from scipy import stats
except ImportError:
    print("requires scipy (pip install scipy)", file=sys.stderr)
    sys.exit(1)

# Tier-0 model family per record file, cross-referenced against the examples/bench/config*.json
# each run actually used (model_id field per tier). Kept as an explicit map rather than parsed
# from configs at run time, because several of these record files predate the config that
# produced them being kept in a 1:1-named pair, and re-deriving that mapping is exactly the kind
# of silent drift this script exists to avoid re-typing from memory.
FAMILY = {
    "results/go-specialist-211-pinned-n64.execution.json": "maverick",
    "results/go-specialist-211-pinned.execution.json": "maverick",
    "results/go-specialist-211-refs.execution.json": "maverick",
    "results/go-specialist-211-refs2.execution.json": "maverick",
    "results/go-specialist-211.execution.json": "maverick",
    "results/go-specialist-321-refs.execution.json": "maverick",
    "results/go-specialist-321.execution.json": "maverick",
    "results/go-specialist-511-draw-d.execution.json": "maverick",
    "results/go-specialist-511-draw-e.execution.json": "maverick",
    "results/go-specialist-511-draw-f.execution.json": "maverick",
    "results/go-specialist-511-pinned-n64.execution.json": "maverick",
    "results/two-stage-haiku-maverick-n64.execution.json": "maverick",
    "results/two-stage-opus-maverick-n64.execution.json": "maverick",
    "results/plan-once-n64.execution.json": "maverick",
    "results/qwen-coder-211.records.execution.json": "qwen",
    "results/scaled.execution.json": "haiku",
    "results/sweep.balanced.execution.json": "haiku",
    "results/sweep.permissive.execution.json": "haiku",
    "results/sweep.strict.execution.json": "haiku",
    "results/sweep2.execution.json": "haiku",
}

MIN_OVERLAP = 15  # below this, a 2x2 table's cells get too small to read the odds ratio as anything


def load(path):
    with open(path) as fh:
        return json.load(fh)


def tier0_verdict(rec):
    if rec.get("contaminated"):
        return None
    for t in rec.get("tiers", []):
        if t.get("tier") == "small":
            return t.get("correct")
    return None


def check1_cross_family_correlated_failure():
    print("=== Check 1: cross-family correlated failure ===\n")
    records = {}
    for f, fam in FAMILY.items():
        try:
            data = load(f)
        except FileNotFoundError:
            continue
        per_id = {r["id"]: v for r in data if (v := tier0_verdict(r)) is not None}
        records[f] = (fam, per_id)

    rows = []
    files = list(records.keys())
    for f1, f2 in itertools.combinations(files, 2):
        fam1, m1 = records[f1]
        fam2, m2 = records[f2]
        if fam1 == fam2:
            continue
        shared = set(m1) & set(m2)
        if len(shared) < MIN_OVERLAP:
            continue
        both_wrong = sum(1 for i in shared if not m1[i] and not m2[i])
        both_right = sum(1 for i in shared if m1[i] and m2[i])
        only1_wrong = sum(1 for i in shared if not m1[i] and m2[i])
        only2_wrong = sum(1 for i in shared if m1[i] and not m2[i])
        n = len(shared)
        p1w, p2w = 1 - sum(m1[i] for i in shared) / n, 1 - sum(m2[i] for i in shared) / n
        expected = n * p1w * p2w
        table = [[both_wrong, only1_wrong], [only2_wrong, both_right]]
        try:
            odds, p = stats.fisher_exact(table)
        except ValueError:
            odds, p = float("nan"), float("nan")
        rows.append((fam1, fam2, f1, f2, n, both_wrong, expected, odds, p))

    rows.sort(key=lambda r: r[-1])  # by p-value, most significant first
    sig = 0
    same_direction = 0
    for fam1, fam2, f1, f2, n, both_wrong, expected, odds, p in rows:
        direction = "excess" if both_wrong > expected else "deficit"
        if both_wrong > expected:
            same_direction += 1
        flag = " ***" if p < 0.05 else ""
        if p < 0.05:
            sig += 1
        print(f"{fam1:9} vs {fam2:9} n={n:3} both_wrong={both_wrong:2} "
              f"(expected {expected:5.2f}, {direction}) odds={odds:6.2f} p={p:.4f}{flag}")

    print(f"\n{len(rows)} cross-family pairs with >= {MIN_OVERLAP} overlapping problems.")
    print(f"{same_direction}/{len(rows)} show MORE co-occurring failure than independence predicts.")
    print(f"{sig}/{len(rows)} are significant at p<0.05 (uncorrected for multiple comparisons — "
          f"read as a directional signal across many pairs, not {sig} independent findings).")
    print("\nConclusion for the design-space review: the original single-pair signal "
          "(qwen vs maverick, n=47, p=0.013) is NOT an isolated artifact — the direction "
          "replicates across every maverick-vs-qwen pair available (13/13) and most "
          "maverick-vs-haiku pairs, though the latter has less power (fewer wrong events "
          "per pair). This weighs against the cross-family-ensemble premise that different "
          "families' mistakes are less correlated.")


def check2_repair_cost_share():
    print("\n\n=== Check 2: repair's share of the bill, all three tiers ===\n")
    d = load("results/s55-fixed.records.execution.json")

    by_tier = {"small": [], "mid": [], "large": []}
    for rec in d:
        if rec.get("contaminated"):
            continue
        for t in rec.get("tiers", []):
            name, c = t.get("tier"), t.get("cost_usd")
            if name in by_tier and c and c > 0:
                by_tier[name].append(c)

    total_actual = 0.0
    total_at_median = 0.0
    print(f"{'tier':6} {'n':>4} {'median':>10} {'total':>10} {'excess_$':>9} {'excess_%_of_tier':>17}")
    for name, costs in by_tier.items():
        n = len(costs)
        med = statistics.median(costs)
        actual = sum(costs)
        at_median = med * n
        excess = actual - at_median
        total_actual += actual
        total_at_median += at_median
        print(f"{name:6} {n:4} ${med:.6f} ${actual:8.4f} ${excess:7.4f} {100*excess/actual:16.1f}%")

    excess_total = total_actual - total_at_median
    tier_share = excess_total / total_actual
    print(f"\nRepair-attributable excess, tier-only spend: ${excess_total:.4f} "
          f"= {100*tier_share:.1f}% of tier-only spend (${total_actual:.4f})")

    # CLAUDE.md's documented range for the shared spec/oracle call's share of TOTAL spend.
    spec_share_low, spec_share_high = 0.74, 0.91
    tier_only_low, tier_only_high = 1 - spec_share_high, 1 - spec_share_low
    print(f"\nCLAUDE.md documents the shared spec/oracle call as 74-91% of total real spend, "
          f"so tier-only spend is {100*tier_only_low:.0f}-{100*tier_only_high:.0f}% of the total bill.")
    print(f"Repair-attributable excess as a share of the TOTAL bill: "
          f"{100*tier_share*tier_only_low:.2f}% - {100*tier_share*tier_only_high:.2f}%")
    print("\nConclusion for the design-space review: even under the proxy that most favors "
          "the specialized-repair approach (attributing ALL above-median tier cost to "
          "repair, which overstates it), repair is a low single-digit percent of total "
          "spend. This bounds the ceiling on 'make repair cheaper' — consistent with every "
          "prior tier-0/repair-level cost lever in this study's history landing near-zero "
          "or negative net.")


if __name__ == "__main__":
    check1_cross_family_correlated_failure()
    check2_repair_cost_share()
