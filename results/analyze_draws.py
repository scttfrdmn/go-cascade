#!/usr/bin/env python3
"""Aggregate several 5:1:1 (and 2:1:1) draws to test experiment 13's confound
empirically: does alpha=0.05 certify with tau0 < 1.0 across fresh draws (i.e. was
scale_chunk luck?), and are the cheap tier's residual wrong answers FLAKY (cluster
splits below the unanimity ceiling) or CONFIDENT (stays at the ceiling)?

The headroom theorem (results/headroom_theorem.py) says fan-out helps iff the errors
are flaky. This measures whether they are, on real draws.

Usage:
    python3 results/analyze_draws.py LABEL=path.execution.json [LABEL=path ...]
e.g.
    python3 results/analyze_draws.py \
        2:1:1=results/go-specialist-211-pinned-n64.execution.json \
        5:1:1-a=results/go-specialist-511-pinned-n64.execution.json \
        5:1:1-b=results/go-specialist-511-draw2.execution.json
"""
import json
import sys


def clean(recs):
    return [r for r in recs
            if not r.get("oracle_unsound") and not r.get("contaminated")]


def truth(t):
    return t.get("true_correct", t.get("correct"))


def analyze(label, path):
    recs = clean(json.load(open(path)))
    n = len(recs)
    ceiling = max((r["tiers"][0]["score"] for r in recs), default=0.0)

    # tier-0 wrong answers, in three classes. The distinction is load-bearing:
    #   - refuted   (score == 0): every sample was refuted by the ladder. A sound,
    #                uniform refutation — escalates trivially. NOT evidence that
    #                fan-out surfaced flakiness (there was no verified cluster at all).
    #   - flaky     (0 < score < ceiling): the samples split — some produced a
    #                verified-but-wrong answer, others differed. THIS is the fan-out
    #                surfacing a non-robust wrong answer as a sub-ceiling cluster.
    #   - confident (score >= ceiling): the wrong answer was reproduced unanimously.
    #                The theorem says no fan-out separates this from a correct one.
    refuted_wrong, flaky_wrong, confident_wrong = [], [], []
    for r in recs:
        t0 = r["tiers"][0]
        if t0.get("correct") is False:
            s = t0["score"]
            if s <= 1e-9:
                refuted_wrong.append((r["id"], round(s, 3)))
            elif s >= ceiling - 1e-9:
                confident_wrong.append((r["id"], round(s, 3)))
            else:
                flaky_wrong.append((r["id"], round(s, 3)))

    # acceptance-risk events at tier 0 (oracle-pass but truly wrong) — the danger.
    risk_events = [r["id"] for r in recs
                   if r["tiers"][0].get("correct") is True
                   and r["tiers"][0].get("true_correct") is False]

    print(f"\n=== {label}  (clean n={n}, tier0 unanimity ceiling {ceiling:.3f}) ===")
    print(f"  tier0 refuted-wrong (score 0, uniform sound refutation):        "
          f"{len(refuted_wrong)} {refuted_wrong}")
    print(f"  tier0 FLAKY-wrong (0 < score < ceiling, fan-out surfaced split): "
          f"{len(flaky_wrong)} {flaky_wrong}")
    print(f"  tier0 CONFIDENT-wrong (score at ceiling, fan-out cannot split):  "
          f"{len(confident_wrong)} {confident_wrong}")
    print(f"  tier0 acceptance-risk events (oracle-pass, truly wrong):         "
          f"{len(risk_events)} {risk_events}")
    return {"label": label, "n": n, "ceiling": ceiling,
            "refuted_wrong": refuted_wrong, "flaky_wrong": flaky_wrong,
            "confident_wrong": confident_wrong, "risk_events": risk_events}


if __name__ == "__main__":
    args = sys.argv[1:]
    if not args:
        print(__doc__)
        sys.exit(1)
    results = []
    for a in args:
        label, path = a.split("=", 1)
        results.append(analyze(label, path))

    print("\n=== confound verdict ===")
    tot_ref = sum(len(r["refuted_wrong"]) for r in results)
    tot_flaky = sum(len(r["flaky_wrong"]) for r in results)
    tot_conf = sum(len(r["confident_wrong"]) for r in results)
    tot_risk = sum(len(r["risk_events"]) for r in results)
    print(f"across {len(results)} draws: {tot_ref} refuted-wrong, {tot_flaky} "
          f"flaky-wrong, {tot_conf} confident-wrong, {tot_risk} acceptance-risk.")
    print("Fan-out helps iff residual tier0 errors are flaky (split) rather than")
    print("confident (at the ceiling). Refuted-wrong (score 0) escalate regardless and")
    print("are not fan-out evidence either way. Confident-wrong at the ceiling are the")
    print("residual danger the theorem says no fan-out removes.")
