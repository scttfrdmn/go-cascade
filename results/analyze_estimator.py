#!/usr/bin/env python3
"""Compute every number the §3.7 estimator write-up quotes, from a records file.

Experiment 19's first write-up computed these ad hoc, which is how it took a second
pass to notice that the oracle behind them had run only 40% of its tests. Making the
analysis a script means the same arithmetic runs over any records file, so the
40%-oracle run and the full-oracle re-run can be compared without hand-reconciling
two sets of hand-computed figures.

    python3 results/analyze_estimator.py results/estimator-n64-full-oracle.json
    python3 results/analyze_estimator.py old.json new.json   # side by side

Everything here is descriptive. None of it is a certificate: the estimator runs off
the acceptance path and moves no threshold (invariants #4/#6).

The statistics, and why each one:

* **eta_fa = |{V=1, Y=0}| / |{V=1, Y observed}|.** The quantity §3.7 estimates by
  mutation score. Rows with no canonical label are excluded from the denominator, not
  counted as successes — a candidate that never got a label is not evidence of
  agreement.
* **Clopper-Pearson one-sided upper bound.** At 0 events this is 1 - delta^(1/n),
  exact rather than normal-approximate, which matters at n in the low hundreds.
* **Sum of (1-M_i) and the Poisson-binomial P(0 events).** The comparison doing the
  real work: if 1-M were a tight estimate of eta_fa, each row would be a Bernoulli
  trial with p_i = 1-M_i, so the expected event count is the sum and P(0) is
  exp(sum log(1-p_i)). This is a comparison *within* one run, so it is robust to the
  small n in a way the per-bucket bounds are not.
* **The M >= 0.90 split.** Pre-registered in the design note. It answers a different
  question from the bound: does M *rank* candidates by eta_fa?
* **The other side of the table.** Confirmed false rejections (the generated suite
  refuted a candidate the canonical suite calls correct) counted separately from rows
  where no candidate survived at all, because the latter carry no label. Pooling them
  reports a false-rejection rate the data does not support.
* **canonical_tests totals.** The audit trail for oracle strength. A run whose
  canonical suites executed far fewer tests than the suites contain measured eta_fa
  against a weakened oracle, which is precisely the defect that made the first run of
  this experiment unusable.
"""

import collections
import json
import math
import sys


def cp_upper(k: int, n: int, delta: float = 0.05) -> float:
    """One-sided Clopper-Pearson upper bound on a binomial rate.

    Closed form at k=0 (1 - delta^(1/n)); bisection on the exact binomial tail
    otherwise. Exact rather than normal-approximate: at k=0 the normal interval is
    degenerate and would report 0, which is the one answer that is certainly wrong.
    """
    if n == 0:
        return float("nan")
    if k == 0:
        return 1.0 - delta ** (1.0 / n)

    def tail(p: float) -> float:
        # P[X <= k] under Binomial(n, p); the bound is the p where this equals delta.
        return sum(math.comb(n, i) * p**i * (1 - p) ** (n - i) for i in range(k + 1))

    lo, hi = 0.0, 1.0
    for _ in range(200):
        mid = (lo + hi) / 2
        if tail(mid) > delta:
            lo = mid
        else:
            hi = mid
    return hi


def analyse(path: str) -> dict:
    rows = json.load(open(path))
    labeled = [r for r in rows if r["generated_accept"] and r["canonical_ran"]]
    events = [r for r in labeled if not r["canonical_correct"]]
    # A candidate existed and the generated suite refuted it, but the canonical suite
    # says it is correct: a confirmed over-rejection.
    false_rej = [r for r in rows if not r["generated_accept"] and r["canonical_ran"] and r["canonical_correct"]]
    no_cand = [r for r in rows if r.get("skipped")]
    # Rows with a candidate the canonical suite could not label (usually an API
    # mismatch). Distinct from no_cand: something was produced, nothing was labeled.
    no_label = [r for r in rows if not r.get("skipped") and not r["canonical_ran"]]

    with_m = [r for r in labeled if r["mutation_valid"] > 0]
    gaps = [1.0 - r["mutation_score"] for r in with_m]
    killed = sum(r["mutation_killed"] for r in with_m)
    valid = sum(r["mutation_valid"] for r in with_m)

    hi = [r for r in with_m if r["mutation_score"] >= 0.90]
    lo = [r for r in with_m if r["mutation_score"] < 0.90]

    ntests = [r.get("canonical_tests", 0) for r in labeled]

    return {
        "path": path,
        "rows": len(rows),
        "problems": len({r["id"] for r in rows}),
        "labeled": len(labeled),
        "events": len(events),
        "event_ids": [(r["id"], r["tier"]) for r in events],
        "eta_fa": len(events) / len(labeled) if labeled else float("nan"),
        "cp95": cp_upper(len(events), len(labeled)),
        "pooled_gap": 1.0 - killed / valid if valid else float("nan"),
        "mutants": (killed, valid),
        "predicted": sum(gaps),
        "p_zero": math.exp(sum(math.log(1 - g) for g in gaps)) if gaps else float("nan"),
        "rows_with_m": len(with_m),
        "hi": (len([r for r in hi if not r["canonical_correct"]]), len(hi), cp_upper(len([r for r in hi if not r["canonical_correct"]]), len(hi))),
        "lo": (len([r for r in lo if not r["canonical_correct"]]), len(lo), cp_upper(len([r for r in lo if not r["canonical_correct"]]), len(lo))),
        "m_undefined": len(labeled) - len(with_m),
        "false_rej": len(false_rej),
        "false_rej_ids": collections.Counter(r["id"] for r in false_rej),
        "with_candidate": len(rows) - len(no_cand),
        "no_cand": len(no_cand),
        "no_label": len(no_label),
        "canon_tests_total": sum(ntests),
        "canon_tests_range": (min(ntests), max(ntests)) if ntests else (0, 0),
    }


def report(a: dict) -> None:
    print(f"=== {a['path']}")
    print(f"  {a['rows']} rows over {a['problems']} problems")
    print(f"  canonical tests executed: {a['canon_tests_total']} total, "
          f"per-row range {a['canon_tests_range'][0]}-{a['canon_tests_range'][1]}")
    print()
    print(f"  eta_fa (V=1 and canonically refuted): {a['events']}/{a['labeled']} = {a['eta_fa']:.4f}")
    print(f"  95% Clopper-Pearson upper bound:      {a['cp95']:.4f}")
    if a["event_ids"]:
        print(f"  events: {a['event_ids']}")
    print()
    print(f"  pooled mutation gap 1-M: {a['pooled_gap']:.4f}  ({a['mutants'][0]}/{a['mutants'][1]} killed)")
    print(f"  events predicted if M were tight (sum of 1-M over {a['rows_with_m']} rows): {a['predicted']:.1f}")
    print(f"  P(0 events | M tight), Poisson-binomial: {a['p_zero']:.3g}")
    print()
    hk, hn, hb = a["hi"]
    lk, ln, lb = a["lo"]
    print(f"  M >= 0.90: {hk}/{hn} = {hk/hn if hn else float('nan'):.4f}  (95% upper {hb:.4f})")
    print(f"  M <  0.90: {lk}/{ln} = {lk/ln if ln else float('nan'):.4f}  (95% upper {lb:.4f})")
    print(f"  M undefined (0 valid mutants): {a['m_undefined']} rows")
    print()
    print(f"  confirmed false rejections: {a['false_rej']} "
          f"({a['false_rej']/a['with_candidate'] if a['with_candidate'] else float('nan'):.3f} of "
          f"{a['with_candidate']} rows that produced a candidate)")
    if a["false_rej_ids"]:
        print(f"    {dict(a['false_rej_ids'])}")
    print(f"  no candidate survived the ladder (unlabeled): {a['no_cand']} rows")
    print(f"  candidate produced but not labelable:         {a['no_label']} rows")
    print()


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    out = [analyse(p) for p in sys.argv[1:]]
    for a in out:
        report(a)
    if len(out) == 2:
        old, new = out
        print("=== side by side (old -> new)")
        for k, fmt in (
            ("canon_tests_total", "{}"),
            ("labeled", "{}"),
            ("events", "{}"),
            ("cp95", "{:.4f}"),
            ("pooled_gap", "{:.4f}"),
            ("predicted", "{:.1f}"),
            ("false_rej", "{}"),
            ("no_cand", "{}"),
        ):
            print(f"  {k:20s} {fmt.format(old[k])} -> {fmt.format(new[k])}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
