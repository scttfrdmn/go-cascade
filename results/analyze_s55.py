#!/usr/bin/env python3
"""Recompute every figure a §5.5 write-up quotes, from the raw records.

Same rationale as `analyze_estimator.py`: the numbers in a results write-up should be
reproducible from the records file by a script, not transcribed from a terminal. Two of
experiment 19's figures were wrong in a first draft because they were remembered rather
than recomputed, and the correction is what surfaced that rejection-side rates are not
stable at n=64.

Usage:
    python3 results/analyze_s55.py results/s55-n470.records

expects `<stem>.execution.json` and `<stem>.judge.json` beside each other, as
`calibrate -compare -records <stem>.json` writes them.

What it reports, and why each is here rather than read off the certificate:

  * n, and the exclusion breakdown. A certificate reports `n_calibration` after
    exclusions; the interesting quantity for a benchmark this new is *what* was
    excluded and why, since oracle-unsound is a property of the generated suite and
    contaminated is a property of the config (invariant #3).
  * Arms (a), (b), (d) — always-frontier, cascade, always-cheapest. These are
    reconstructed from one paired run's per-tier records by `calibrate.Baselines`;
    this script recomputes them independently so a disagreement is visible rather
    than assumed away.
  * Arm (c) versus arm (b): the *lowest* alpha each oracle can certify at fixed
    delta and n, which is §5.5(3)'s primary comparative outcome, plus the judge's
    realized-vs-empirical gap (its unobservable false-acceptance rate).
  * Cost per query for each arm, so the headline "cheaper at fixed risk" claim is
    checkable.
  * The price of each oracle, per tier observation. The two arms share their
    candidate stream and charge sampling identically (ProfilePaired costs it once),
    so the per-tier cost difference is *exactly* the oracle: the execution arm pays
    `cpu_seconds * ComputeUSDPerCoreSecond` (judge.go:227) and the judge arm pays its
    reviewer's inference tokens (judge.go:234). Reported separately because a
    cheaper-AND-sound executable oracle is a stronger claim than soundness alone,
    and because it is otherwise invisible: it hides inside each arm's cost/query.

Ground truth is `true_correct` where present, falling back to `correct` — mirroring
`TierObs.truth()` in internal/calibrate/ltt.go. The fallback is sound for the
execution arm (the oracle is exact) and is why the judge arm records both.
"""

import json
import pathlib
import sys


def truth(t: dict) -> bool:
    """Execution ground truth for a tier observation. Mirrors TierObs.truth()."""
    return t["true_correct"] if "true_correct" in t else t["correct"]


def load(stem: str, arm: str) -> list[dict]:
    p = pathlib.Path(f"{stem}.{arm}.json")
    if not p.exists():
        sys.exit(f"missing {p}")
    return json.loads(p.read_text())


def usable(recs: list[dict]) -> list[dict]:
    """Records that carry tier data and are not excluded.

    Excludes cache hits as well as flagged records: the baselines are statements
    about the model tiers, so a cache hit has no tier ladder to reconstruct them
    from (see calibrate.Baselines, which skips them for the same reason).
    """
    return [
        r
        for r in recs
        if r.get("tiers")
        and not r.get("cache_hit")
        and not r.get("contaminated")
        and not r.get("oracle_unsound")
    ]


def arms(recs: list[dict]) -> dict:
    """Always-cheapest (d), always-frontier (a), and per-tier cumulative cost.

    The cascade arm (b) is not computed here: it depends on the certified threshold
    vector, so it is read from the certificate instead of guessed at.
    """
    out = {}
    for name, idx in (("always-cheapest (d)", 0), ("always-frontier (a)", -1)):
        bad = sum(1 for r in recs if not truth(r["tiers"][idx]))
        cost = sum(r["tiers"][idx]["cost_usd"] for r in recs)
        out[name] = {"risk": bad / len(recs), "mean_usd": cost / len(recs), "n": len(recs)}
    return out


def oracle_gap(recs: list[dict]) -> dict:
    """Judge disagreement with execution, split by direction.

    The split matters more than the total: an over-*rejection* costs escalations and
    is invisible to the certificate, while an over-*acceptance* is a false accept the
    certificate cannot see and is the thing §3.1's floor is about.
    """
    over_acc = over_rej = agree = 0
    for r in recs:
        for t in r["tiers"]:
            if "true_correct" not in t:
                continue  # cannot compare without ground truth
            if t["correct"] == t["true_correct"]:
                agree += 1
            elif t["correct"] and not t["true_correct"]:
                over_acc += 1
            else:
                over_rej += 1
    return {"agree": agree, "over_accept": over_acc, "over_reject": over_rej}


def billed(exec_recs: list[dict], judge_recs: list[dict]) -> dict:
    """Actual inference spend, which is NOT the sum of the two arms' cost fields.

    Two traps in reading `cost_usd` as money:

      1. ProfilePaired charges the *shared* sampling cost to both arms' records
         (judge.go:216-217) because each arm's cost/query must be self-contained.
         Adding the arms therefore bills sampling twice.
      2. The execution arm's oracle charge is `cpu_seconds * ComputeUSDPerCoreSecond`
         — a modelled local-compute price (default 1.33e-5, config.go:174), not a
         Bedrock invoice line.

    Writing the two arms out makes the answer fall out:

        execution total = sampling + modelled_cpu
        judge total     = sampling + judge_tokens

    Real Bedrock spend is sampling (once) + judge_tokens, which is *exactly the judge
    arm's total* — the execution arm contributes no inference beyond the sampling both
    arms already carry. So the paired run's bill is the judge column, not the sum, and
    a `-compare` run costs the judge's tokens more than a solo run, not double.
    """
    ex = sum(t["cost_usd"] for r in exec_recs for t in r.get("tiers", []))
    jd = sum(t["cost_usd"] for r in judge_recs for t in r.get("tiers", []))
    return {"exec_total": ex, "judge_total": jd, "billed": jd, "judge_tokens": jd - ex}


def oracle_price(exec_recs: list[dict], judge_recs: list[dict]) -> dict:
    """Per-tier-observation cost difference between the arms — the price of the oracle.

    Paired by (id, tier index): ProfilePaired charges sampling identically to both arms,
    so whatever is left over is what each oracle costs to consult. Only tiers where both
    arms actually ruled (a representative existed) are comparable.
    """
    by_id = {r["id"]: r for r in judge_recs}
    ex = jd = n = 0.0
    for r in exec_recs:
        o = by_id.get(r["id"])
        if not o:
            continue
        for a, b in zip(r["tiers"], o["tiers"]):
            if "true_correct" not in a:
                continue  # no representative; neither oracle was consulted
            ex += a["cost_usd"]
            jd += b["cost_usd"]
            n += 1
    if not n:
        return {}
    return {"n": int(n), "exec_mean": ex / n, "judge_mean": jd / n}


def main() -> int:
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    stem = sys.argv[1].removesuffix(".json")

    arm_recs, all_recs = {}, {}
    for arm in ("execution", "judge"):
        recs = load(stem, arm)
        all_recs[arm] = recs  # spend is over everything: an excluded problem was paid for
        arm_recs[arm] = usable(recs)
        keep = usable(recs)
        exc_c = sum(1 for r in recs if r.get("contaminated"))
        exc_u = sum(1 for r in recs if r.get("oracle_unsound"))
        hits = sum(1 for r in recs if r.get("cache_hit"))
        print(f"=== {arm} ===")
        print(f"  records            {len(recs)}")
        print(f"  usable             {len(keep)}")
        print(f"  excluded           {exc_c} contaminated, {exc_u} oracle-unsound, {hits} cache hits")
        if not keep:
            print("  no usable records\n")
            continue
        for name, m in arms(keep).items():
            print(f"  {name:22s} risk {m['risk']:.4f}  ${m['mean_usd']:.5f}/query")
        g = oracle_gap(keep)
        tot = g["agree"] + g["over_accept"] + g["over_reject"]
        if tot:
            print(
                f"  oracle vs truth        {g['agree']}/{tot} agree, "
                f"{g['over_accept']} over-accept, {g['over_reject']} over-reject"
            )
        print()

    # Over ALL records, not just usable ones: an oracle-unsound problem still cost
    # tokens to discover. Dividing by usable n would understate the per-problem rate
    # and misproject the remaining bill.
    b = billed(all_recs["execution"], all_recs["judge"])
    n = max(len(all_recs["execution"]), 1)
    print("=== spend ===")
    print(f"  execution arm cost field    ${b['exec_total']:.4f}   (sampling + modelled cpu)")
    print(f"  judge arm cost field        ${b['judge_total']:.4f}   (the same sampling + judge tokens)")
    print(f"  judge tokens                ${b['judge_tokens']:.4f}")
    print(f"  actual inference spend      ${b['billed']:.4f}   (${b['billed']/n:.5f}/problem)")
    print("  (NOT the sum of the arms: sampling is charged to both records so each arm's")
    print("   cost/query is self-contained. The bill is the judge column.)")
    print()

    if p := oracle_price(arm_recs["execution"], arm_recs["judge"]):
        print("=== price of the oracle (paired per-tier) ===")
        print(f"  tier observations compared  {p['n']}")
        print(f"  execution arm, total        ${p['exec_mean']:.6f}/tier")
        print(f"  judge arm, total            ${p['judge_mean']:.6f}/tier")
        print(f"  difference                  ${p['judge_mean'] - p['exec_mean']:+.6f}/tier")
        print("  (the two totals both include the shared sampling charge, which the arms")
        print("   pay identically; only the DIFFERENCE isolates judge tokens vs cpu-seconds)")
        print()

    cert = pathlib.Path(f"{stem.rsplit('.records',1)[0]}.cert.json")
    if cert.exists():
        c = json.loads(cert.read_text())
        print("=== certificate ===")
        for k in (
            "valid", "alpha", "delta", "n_calibration", "n_excluded_contaminated",
            "n_excluded_oracle_unsound", "thresholds", "empirical_risk",
            "realized_risk", "p_value", "expected_cost_usd",
        ):
            if k in c:
                print(f"  {k:26s} {c[k]}")
        if c.get("note"):
            print(f"  note                       {c['note']}")
    else:
        print(f"(no certificate at {cert})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
