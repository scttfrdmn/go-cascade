#!/usr/bin/env python3
"""How much of a benchmark can a warm cache absorb? Measured exactly, offline, free.

This exists because §5.5(4) (cache-warmth sensitivity, the direct §2.9 test) was about
to be run live at ~$7 without first checking whether the benchmark can exhibit the
effect at all. It cannot, and that is decidable without spending anything.

Two quantities, and the gap between them is the finding:

  * **Retrieval candidacy** — how many problems have a prior problem above
    `cache_min_similarity`. Retrieval is Jaccard overlap on character trigrams
    (`cache.Similarity`, internal/cache/canonical.go), so this is pure text.

  * **Absorption** — how many queries a warm cache would actually serve. Arm zero
    re-executes a retrieved solution against the *new* query's tests and keeps it only
    if it passes (invariant #5: verified, never predicted). So absorption requires the
    donor's solution to satisfy the recipient's suite.

Measured on MultiPL-E Go (488 problems, ~/mple-bench):

    retrieval candidacy at min_sim=0.35   464/488  (95.1%)
    absorption ceiling                      2/488  ( 0.4%)

Three orders of magnitude apart, and that is the substantive result: on this benchmark
lexical similarity is almost uninformative about transferability. It is also the
reason §5.5(4) cannot be run here — a cache that absorbs 0.4% of traffic induces no
measurable distribution shift, so there is no §2.9 effect to be sensitive to, and a
paid run would report a null caused by the benchmark rather than by the method.

Worse than uninformative, at the high end it is *anti*-correlated. The most-similar
pairs in the benchmark are antonyms: `mbpp_404_minimum`~`mbpp_309_maximum` at **0.949**
(the single highest pair in 118,828), `first_Digit`~`last_Digit` at 0.937,
`even_position`~`odd_position` at 0.920, `remove_lowercase`~`remove_uppercase` at 0.915.
Near-identical text, opposite required answers. A cache with a similarity threshold and
no re-execution would be most confidently wrong exactly where it is most sure — which is
invariant #5's argument, measured rather than asserted.

The absorption number is a **ceiling**, not an estimate, and is exact rather than
sampled. Two free filters make exhaustive measurement cheap:

  1. A donor whose signature differs from the recipient's cannot compile against the
     recipient's suite, so it is a guaranteed refutation. `manifest.json` pins every
     signature, so this filter costs nothing and eliminates all but 5 groups.
  2. Within a signature group, every ordered (donor, recipient) pair is executed. Ten
     pairs, so no sampling is needed.

It is a ceiling and not the realized rate because it uses each problem's *reference*
solution as the donor. A real cache stores whatever the router accepted, which is at
best as good as the reference. It also ignores arrival order (a cache only absorbs a
query if the donor arrived first) and the fact that the spec phase runs *before*
`tryCache` (cascade.go:171 vs 181), so even a hit pays for spec generation.

Result: 4 of 10 same-signature transfers pass, forming 2 bidirectional pairs —
`mbpp_102`/`mbpp_411` (snake_to_camel) and `mbpp_591`/`mbpp_625` (swap_List). Both are
upstream MBPP shipping one task twice under different ids, with reworded statements and
different test tables: exactly the case a verified cache should serve. Every other
near-duplicate is retrieved and refuted, including the highest-similarity
same-signature pair in the set — `he_56`/`he_61` `CorrectBracketing` at 0.836, where one
problem is about `<>` and the other about `()`. A similarity-threshold cache would have
returned a wrong answer there; re-execution catches it. That single pair is the
strongest available argument for invariant #5 on real data.

Usage:
    python3 results/absorption_ceiling.py [~/mple-bench]

Needs a `go` toolchain on PATH (it executes candidate/suite pairs) and the benchmark
built by examples/bench/multipl/. Takes ~1 minute, dominated by the similarity sweep.
"""

import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile


def normalize(s: str) -> str:
    """Port of cache.NormalizeProblem: lowercase, collapse whitespace.

    Not optional — `trigrams` applies it before tokenising (canonical.go:163), so a port
    that skips it reports similarities the router would never compute.
    """
    return " ".join(s.split()).lower()


def trigrams(s: str) -> set[str]:
    """Character trigrams over the normalized text. Mirrors cache.trigrams."""
    r = normalize(s)
    return {r[i : i + 3] for i in range(max(len(r) - 2, 0))}


def similarity(a: str, b: str) -> float:
    """Jaccard overlap on character trigrams — a port of cache.Similarity.

    Reimplemented in Python rather than shelled out to Go so the sweep is one process
    instead of 119k. Ported code is a place to introduce a silent discrepancy, so
    `check_port` validates it against the Go original before any number is reported.
    """
    ta, tb = trigrams(a), trigrams(b)
    if not ta or not tb:
        return 0.0
    return len(ta & tb) / len(ta | tb)


def check_port(text: dict[str, str], ids: list[str], repo: pathlib.Path) -> None:
    """Validate the Python similarity port against internal/cache's Go implementation.

    Cheap insurance: a port that disagrees would silently restate the headline candidacy
    figure as this script's artifact rather than the router's behaviour. Samples pairs
    spanning the range instead of all 119k, and hard-fails on disagreement.
    """
    pairs = [(ids[i], ids[j]) for i, j in ((0, 1), (0, 40), (5, 6), (60, 61), (100, 300), (7, 29))]
    prog = """package main
import ("bufio";"encoding/json";"fmt";"os"
 "github.com/scttfrdmn/go-cascade/internal/cache")
func main(){
 sc:=bufio.NewScanner(os.Stdin); sc.Buffer(make([]byte,1<<22),1<<22)
 for sc.Scan(){
  var p [2]string
  if err:=json.Unmarshal(sc.Bytes(),&p); err!=nil { panic(err) }
  fmt.Printf("%.12f\\n", cache.Similarity(p[0],p[1]))
 }
}"""
    # The helper must live inside the module: `internal/` is not importable from a
    # package outside it, so a /tmp scratch dir cannot compile against cache.Similarity.
    with tempfile.TemporaryDirectory(dir=repo) as d:
        main = pathlib.Path(d) / "main.go"
        main.write_text(prog)
        stdin = "\n".join(json.dumps([text[a], text[b]]) for a, b in pairs)
        p = subprocess.run(
            ["go", "run", str(main)],
            cwd=repo, input=stdin, capture_output=True, text=True, timeout=180,
        )
    if p.returncode != 0:
        sys.exit(f"could not cross-check the similarity port against Go:\n{p.stderr}")
    got = [float(x) for x in p.stdout.split()]
    for (a, b), g in zip(pairs, got):
        mine = similarity(text[a], text[b])
        if abs(mine - g) > 1e-9:
            sys.exit(
                f"similarity port disagrees with cache.Similarity on {a}~{b}: "
                f"python {mine:.12f} vs go {g:.12f}"
            )
    print(f"similarity port cross-checked against internal/cache on {len(pairs)} pairs: exact\n")


def load(base: pathlib.Path) -> tuple[list[str], dict[str, str], dict[str, dict]]:
    rows = [json.loads(l) for l in (base / "problems.jsonl").read_text().splitlines() if l.strip()]
    ids = [r["id"] for r in rows]
    text = {r["id"]: r["problem"] for r in rows}
    man = {m["id"]: m for m in json.loads((base / "manifest.json").read_text())}
    return ids, text, man


def candidacy(ids: list[str], text: dict[str, str], thresholds: list[float]) -> None:
    """Retrieval candidacy as a cache warms in stream order.

    For each problem, the best similarity against anything already seen — which is what
    `Retrieve` ranks on. Reported at several thresholds because the answer is extremely
    threshold-sensitive and quoting one number would hide that.
    """
    best_prior = []
    for i, cur in enumerate(ids):
        top = max((similarity(text[cur], text[p]) for p in ids[:i]), default=0.0)
        best_prior.append(top)
    print("retrieval candidacy (has any prior problem above the floor):")
    for t in thresholds:
        c = sum(1 for b in best_prior if b >= t)
        print(f"  min_sim={t:.2f}   {c:3d}/{len(ids)}  ({100*c/len(ids):.1f}%)")
    print(f"  max similarity between any two problems: {max(best_prior):.3f}")


def top_pairs(ids: list[str], text: dict[str, str], man: dict[str, dict], k: int = 12) -> None:
    """The most-similar pairs in the benchmark, annotated with signature agreement.

    Two jobs. First, it audits the signature pre-filter: if the top of the similarity
    distribution were full of same-signature pairs the filter skipped, the ceiling below
    would be an undercount. It is not — the filter only ever drops guaranteed
    refutations.

    Second, it is the more interesting result on its own. The top of this distribution is
    dominated by *antonym* pairs — `minimum`/`maximum` at 0.949, `first_Digit`/
    `last_Digit`, `even_position`/`odd_position`, `remove_lowercase`/`remove_uppercase`.
    Near-identical text, opposite semantics. So on this benchmark lexical similarity is
    not merely a weak predictor of transferability, it is *anti*-correlated with it at
    the high end: a threshold cache is most confidently wrong exactly where it is most
    sure. That is invariant #5's whole argument, on real data.
    """
    scored = []
    for i in range(len(ids)):
        for j in range(i):
            scored.append((similarity(text[ids[i]], text[ids[j]]), ids[i], ids[j]))
    scored.sort(reverse=True)
    print(f"\ntop {k} pairs by similarity (audits the signature pre-filter below):")
    for s, a, b in scored[:k]:
        same = man[a]["sig"] == man[b]["sig"]
        print(f"  {s:.3f} {'SAME-SIG' if same else 'diff-sig'}  {a} ~ {b}")
        if not same:
            print(f"           {man[a]['sig']}")
            print(f"           {man[b]['sig']}")


def absorption(base: pathlib.Path, ids: list[str], man: dict[str, dict]) -> None:
    """Exact absorption ceiling: execute every same-signature transfer."""
    bysig: dict[str, list[str]] = {}
    for i in ids:
        bysig.setdefault(man[i]["sig"], []).append(i)
    groups = [v for v in bysig.values() if len(v) > 1]
    npairs = sum(len(v) * (len(v) - 1) for v in groups)
    covered = sum(len(v) for v in groups)
    print(
        f"\nsignature groups with >1 member: {len(groups)} "
        f"({covered} problems, {npairs} ordered transfers to execute)"
    )
    print("every other pair is a guaranteed refutation: a differing signature cannot compile.\n")

    absorbed, tested, skipped = [], 0, 0
    for g in groups:
        for donor in g:
            src = base / "refs" / donor / "solution.go"
            if not src.exists():
                skipped += 1  # unvalidated reference; cannot act as a donor
                continue
            for recip in g:
                if donor == recip:
                    continue
                with tempfile.TemporaryDirectory() as d:
                    shutil.copy(base / "refs" / recip / "solution_test.go", d)
                    shutil.copy(base / "refs" / recip / "go.mod", d)
                    shutil.copy(src, pathlib.Path(d) / "solution.go")
                    p = subprocess.run(
                        ["go", "test", "-count=1", "./..."],
                        cwd=d, capture_output=True, text=True, timeout=180,
                    )
                tested += 1
                ok = p.returncode == 0
                print(f"  {'ABSORB ' if ok else 'refuted'}  {donor} -> {recip}")
                if ok:
                    absorbed.append((donor, recip))

    uniq = {frozenset(p) for p in absorbed}
    print(f"\n{len(absorbed)}/{tested} transfers succeed ({len(uniq)} distinct pairs)")
    if skipped:
        print(f"({skipped} donors skipped: no validated reference)")
    print(f"absorption ceiling: {len(uniq)}/{len(ids)} queries ({100*len(uniq)/len(ids):.1f}%)")
    print(
        "\nCeiling, not realized rate: donors are reference solutions (a real cache stores\n"
        "what the router accepted), arrival order is ignored, and the spec phase runs before\n"
        "tryCache so even a hit pays for spec generation."
    )


def main() -> int:
    base = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser("~/mple-bench"))
    if not (base / "problems.jsonl").exists():
        sys.exit(f"no benchmark at {base} (build it with examples/bench/multipl/)")
    ids, text, man = load(base)
    print(f"benchmark: {base}  ({len(ids)} problems)\n")
    check_port(text, ids, pathlib.Path(__file__).resolve().parent.parent)
    candidacy(ids, text, [0.35, 0.50, 0.60, 0.70])
    top_pairs(ids, text, man)
    absorption(base, ids, man)
    return 0


if __name__ == "__main__":
    sys.exit(main())
