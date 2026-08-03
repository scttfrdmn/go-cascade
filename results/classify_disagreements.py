#!/usr/bin/env python3
"""Dump the programs behind each judge/execution disagreement, for defect classification.

Experiment 21 measured eta_fa for the first time — 11 over-acceptances in 1096 tier
observations, with the cheap-tier gradient §3.1 predicts — and then could not say what
any of the 11 programs got *wrong*, because `TierObs` stored verdicts and no source.
The paper's claim is specifically that the judge's blind spot is **reading-invisible**
defects; without the programs, that mechanism stays argued. Issue #49 added
`disagreement_source`, retained only where an arm's oracle disagrees with execution
truth. This script reads it back out.

Usage:
    python3 results/classify_disagreements.py results/<stem>.records.judge.json
    python3 results/classify_disagreements.py results/<stem>.records.judge.json --dump out/

The summary alone answers "how many, which direction, which tier". `--dump` writes one
.go file per disagreement so the programs can actually be read — which is the point, and
which no aggregate can substitute for.

Reading guidance, learned from experiment 21:

  * **Over-acceptance is the dangerous direction** (eta_fa): the judge passed a program
    execution refutes, so it inflates true risk above the certified bound invisibly.
    Over-*rejection* costs escalations, not risk — it is why the judge loses the
    certification race, but it is safe.
  * **Quote the denominator.** Rates here are over observations that carry
    `true_correct`; observations without it are skipped, never counted as agreement.
    For the judge arm, falling back to `correct` would force agreement and silently
    zero out the very thing being measured.
  * A tier gradient (worse on cheap tiers) is expected and is not itself the mechanism
    claim. The mechanism claim needs the *defect classes* — which is what reading the
    dumped programs is for.

Records written before issue #49 carry no sources; this script says so rather than
reporting zero disagreements.
"""

import collections
import json
import pathlib
import sys


def load(path: str) -> list[dict]:
    p = pathlib.Path(path)
    if not p.exists():
        sys.exit(f"missing {p}")
    return json.loads(p.read_text())


def disagreements(recs: list[dict]) -> list[dict]:
    """Every observation where the arm's oracle differs from execution truth.

    Mirrors TierObs.Disagrees(): an observation with no `true_correct` has nothing to
    compare against and is skipped, NOT treated as agreement.
    """
    out = []
    for r in recs:
        if r.get("oracle_unsound") or r.get("contaminated"):
            continue  # excluded from calibration, so not part of any reported rate
        for i, t in enumerate(r["tiers"]):
            if "true_correct" not in t or t["correct"] == t["true_correct"]:
                continue
            out.append({
                "id": r.get("id", "?"),
                "tier": t.get("tier", f"tier{i}"),
                "tier_index": i,
                "direction": "over_accept" if t["correct"] else "over_reject",
                "score": t.get("score"),
                "source": t.get("disagreement_source", ""),
            })
    return out


def main() -> int:
    # Hand-rolled rather than argparse to match the other scripts here, but note
    # --dump's VALUE is not flag-prefixed, so it must be consumed with the flag
    # instead of filtered out as a positional.
    argv, positional, dump_dir = sys.argv[1:], [], None
    i = 0
    while i < len(argv):
        if argv[i] == "--dump":
            if i + 1 >= len(argv):
                sys.exit("--dump needs a directory")
            dump_dir = pathlib.Path(argv[i + 1])
            i += 2
            continue
        if argv[i].startswith("--"):
            sys.exit(f"unknown flag {argv[i]}")
        positional.append(argv[i])
        i += 1
    if len(positional) != 1:
        sys.exit(__doc__)
    args = positional

    recs = load(args[0])
    ds = disagreements(recs)

    comparable = sum(
        1
        for r in recs
        if not (r.get("oracle_unsound") or r.get("contaminated"))
        for t in r["tiers"]
        if "true_correct" in t
    )
    print(f"records                {len(recs)}")
    print(f"comparable observations {comparable}   (carry true_correct; others skipped)")
    print(f"disagreements          {len(ds)}")
    if comparable:
        acc = sum(1 for d in ds if d["direction"] == "over_accept")
        rej = len(ds) - acc
        print(f"  over-accept (eta_fa) {acc}/{comparable}   <- the dangerous direction")
        print(f"  over-reject (beta)   {rej}/{comparable}   <- costs escalations, not risk")

    by_tier = collections.Counter((d["tier"], d["direction"]) for d in ds)
    if by_tier:
        tiers = sorted({t for t, _ in by_tier})
        print("\nby tier")
        print(f"  {'tier':12s} {'over-accept':>12s} {'over-reject':>12s}")
        for t in tiers:
            print(f"  {t:12s} {by_tier[(t,'over_accept')]:12d} {by_tier[(t,'over_reject')]:12d}")

    with_src = [d for d in ds if d["source"]]
    print(f"\nwith retained source   {len(with_src)}/{len(ds)}")
    if ds and not with_src:
        print("  NOTE: these records predate issue #49, so no program text was kept and")
        print("  the defect classes are NOT recoverable from this file. Re-run to classify.")

    if dump_dir and with_src:
        dump_dir.mkdir(parents=True, exist_ok=True)
        for n, d in enumerate(with_src):
            safe = "".join(c if c.isalnum() or c in "-_" else "_" for c in d["id"])
            p = dump_dir / f"{d['direction']}-{safe}-{d['tier']}-{n}.go"
            p.write_text(
                f"// {d['direction']} | id={d['id']} tier={d['tier']} score={d['score']}\n"
                f"// The judge's verdict differed from execution truth on this program.\n"
                f"// Classify the defect: is it reading-INvisible (race, overflow, aliasing,\n"
                f"// boundary) or should a careful reviewer have caught it?\n\n"
                + d["source"]
            )
        print(f"wrote {len(with_src)} programs to {dump_dir}/")
    elif dump_dir:
        print("nothing to dump")

    # Over-acceptances first: they are the ones §3.1 is about.
    for d in sorted(ds, key=lambda d: (d["direction"] != "over_accept", d["tier"])):
        src = f"{len(d['source'])}B" if d["source"] else "NO SOURCE"
        print(f"  {d['direction']:12s} {d['id']:34s} {d['tier']:8s} score={d['score']} {src}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
