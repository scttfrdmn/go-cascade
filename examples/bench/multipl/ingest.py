#!/usr/bin/env python3
"""Ingest MultiPL-E's Go variants (HumanEval-Go, MBPP-Go) into the layout the
go-cascade benchmarks use, so the §5.5 validation experiment can run at n >= 300
on a *standard* benchmark instead of this project's 64 hand-written problems.

What this produces, per problem:

    problems.jsonl                 {"id":..., "problem":...}  — the query stream
    manifest.json                  [{"id","fn","sig","ret"}]  — signatures for stage 2
    refs/<id>/go.mod               module bench/<id>
    refs/<id>/solution_test.go     MultiPL-E's human-derived suite, rewritten
    refs/<id>/solution.go          reference implementation  (NOT written here)

The reference implementations are deliberately *not* produced by this script —
MultiPL-E ships prompts and tests but no solutions. `stage2_references.py`
generates one per problem with a frontier model and keeps only those that pass
MultiPL-E's own tests, which is why the reference is trustworthy without a human
writing it. Ingestion and reference generation are separate steps so this one
stays free, deterministic, and re-runnable.

Shape of the upstream data, verified across all 528 rows rather than assumed:
exactly one function per prompt, exactly one `TestX` function per suite (all cases
live in a `[]test` table driven by `t.Run` subtests), no helper functions, no
declared types, and `fmt`+`testing` are the only imports any suite needs.

Three deliberate divergences from upstream MultiPL-E, all recorded in README.md:

1. **Exported names.** MultiPL-E emits snake_case (`has_close_elements`), which
   in Go is unexported. `prompt.ExtractAPI` keeps only *exported* declarations,
   so `-pin-api` would extract an empty API and the oracle-soundness gate could
   never reach a verdict. We rename to CamelCase (`HasCloseElements`) in the
   prompt, the docstring's doctest lines, and the test suite. The alternative —
   teaching ExtractAPI about unexported functions — would change what `-pin-api`
   does to the 64-problem results every prior pinned experiment reported, so the
   rename lives here instead of in an invariant-carrying package.

2. **Package name.** MultiPL-E uses `package <fn>_test`; the go-cascade verifier
   expects `package solution` (it writes a candidate `solution.go` next to the
   suite in one directory). Rewritten accordingly.

3. **Test function name: `TestCanonical`.** Upstream names it after the function
   (`TestHas_Close_Elements`), and that name is actively dangerous here because it
   begins with "TestH": the ladder's acceptance stage filters on `^TestH` and its
   visible stage on `^TestV`, so a canonical suite fed to either would run a
   silent *fraction* of itself. That is not hypothetical — it is exactly the bug
   that made experiment 19's first run measure eta_fa against 40% of its oracle.
   `TestCanonical` matches neither filter, so any future mis-routing trips the
   zero-tests-ran guard in `ladder.Run`/`Accept` and fails loudly instead of
   quietly weakening the oracle. The canonical suite is consumed by
   `Ladder.RunAllTests`, which applies no filter, so nothing needs the V/H split.

The problem statement handed to the model is MultiPL-E's docstring, with the
`>>> ` doctest lines kept: they are part of the published prompt, and dropping
them would make this an easier benchmark than the one everyone else reports. The
MBPP statements say "write a gothon function" — a translation artifact in the
upstream dataset. It is left verbatim: editing the published prompt would make
results incomparable to everyone else's, which costs more than the oddity does.
"""

import argparse
import concurrent.futures as cf
import json
import pathlib
import re
import shutil
import subprocess
import sys

# One function per prompt, always; verified across all 528 rows before relying on
# it. A prompt that does not match this shape is skipped and reported rather than
# guessed at. The return type is captured separately from the parameters because
# the signature has to be reassembled, and gluing them back together wrongly is
# how the first draft of this script emitted `func F(x int) bool)`.
#
# `ret` must NOT exclude braces: 53 of the 528 problems return `[]interface{}`,
# and a `[^{]*?` return group silently drops every one of them. It is bounded to a
# single line instead, which is enough — the signature is always one line, and the
# lazy match then stops at the opening brace of the body because `\{\s*$` anchors
# it there.
FUNC_RE = re.compile(r"^func\s+(\w+)\s*\((?P<params>[^\n]*?)\)\s*(?P<ret>[^\n]*?)\s*\{\s*$", re.M)
DOC_RE = re.compile(r"((?:^//.*\n)+)\s*func\s+\w+\s*\(", re.M)
# The single upstream test function, whatever it is called.
TESTFN_RE = re.compile(r"^func\s+Test\w*\s*\(t \*testing\.T\)\s*\{", re.M)

CANONICAL_TEST = "TestCanonical"

# One `{actual: candidate(...), expected: ...},` row of the suite's test table.
# Non-greedy and anchored on `), expected:` because arguments themselves contain
# parentheses and commas; the trailing `},` bounds the expectation.
CALL_RE = re.compile(r"candidate\((.*?)\),\s*expected:\s*(.*?)\},", re.S)
NUM_RE = re.compile(r"-?\b\d+\.?\d*\b")


def _norm_literals(s: str) -> str:
    """Collapse whitespace and rewrite numeric literals to a canonical value form, so
    `3` and `3.0` compare equal. Used only by `contradiction` — see the reasoning
    there for why comparing by value rather than spelling is the sound choice."""

    def by_value(m: re.Match[str]) -> str:
        try:
            return repr(float(m.group(0)))
        except ValueError:  # not actually numeric (e.g. part of an identifier)
            return m.group(0)

    return NUM_RE.sub(by_value, " ".join(s.split()))


def camel(snake: str) -> str:
    """has_close_elements -> HasCloseElements; differ_At_One_Bit_Pos -> DifferAtOneBitPos.

    MBPP mixes cases inside a single name, so each part is lowercased before
    capitalising: without that, `differ_At_One_Bit_Pos` would come out as
    `DifferAtOneBitPos` only by luck and `is_NOT_prime` would keep its shouting.
    """
    return "".join(p[:1].upper() + p[1:].lower() if p else "" for p in snake.split("_"))


def rename_ident(src: str, old: str, new: str) -> str:
    """Replace whole-word `old` with `new`. Word boundaries matter: `is_prime`
    must not match inside `is_prime_helper`."""
    return re.sub(r"\b" + re.escape(old) + r"\b", new, src)


def problem_id(name: str) -> str:
    """HumanEval_0_has_close_elements -> he_0_has_close_elements; mbpp_3_is_not_prime
    -> mbpp_3_is_not_prime. Keeps the upstream number so a result can always be
    traced back to the published problem."""
    if name.startswith("HumanEval_"):
        return "he_" + name[len("HumanEval_") :]
    return name


def signature(fn: str, params: str, ret: str) -> str:
    """Reassemble a full Go signature, parenthesising the parameter list exactly
    once. Written as one function so the prompt and the manifest cannot disagree
    about what the pinned API is — they are the same string."""
    params = params.strip()
    ret = ret.strip()
    return f"func {fn}({params})" + (f" {ret}" if ret else "")


def statement(doc: str, fn_snake: str, fn_camel: str, sig: str) -> str:
    """Turn the Go doc comment into a prose problem statement.

    The exported name and full signature are stated explicitly because the
    benchmark is *pinned*: the generated tests and the reference both use this
    exact signature, so a candidate that invents its own name fails to compile
    and would be scored as wrong for the wrong reason. The docstring's own
    `>>> has_close_elements(...)` examples are renamed too — leaving the old name
    there tells the model, in the same breath, to implement two different
    identifiers.

    Only the *pinned* identifier is renamed, deliberately. Three statements still
    contain snake_case prose (`brazilian_factorial(n) = n! * ...` in
    he_139_special_factorial is a mathematical definition, not the function being
    asked for); rewriting those would alter the published problem text for no gain.
    """
    body = "\n".join(ln[2:].lstrip() if ln.startswith("//") else ln for ln in doc.strip().splitlines())
    body = rename_ident(body, fn_snake, fn_camel)
    return (
        f"{body}\n\n"
        f"Implement this as an exported Go function with exactly this signature:\n"
        f"    {sig}\n"
        f"Use only the standard library."
    )


def ingest_row(row: dict) -> dict | None:
    """Return the ingested artifacts for one MultiPL-E row, or None if its shape
    is unexpected (reported by the caller rather than silently dropped)."""
    prompt, tests = row["prompt"], row["tests"]
    m = FUNC_RE.search(prompt)
    if not m:
        return None
    fn_snake, params, ret = m.group(1), m.group("params"), m.group("ret")
    fn = camel(fn_snake)
    dm = DOC_RE.search(prompt)
    if not dm:
        return None
    if not TESTFN_RE.search(tests):
        return None

    sig = signature(fn, params, ret)
    # MultiPL-E's suite calls the function once, via `candidate := <fn>`. Renaming
    # the identifier there is the whole edit; nothing else references it.
    suite = rename_ident(tests, fn_snake, fn)
    # Rename the test function itself — see divergence 3 in the module docstring:
    # the upstream name starts with "TestH" and would be half-run by the ladder's
    # acceptance filter.
    suite = TESTFN_RE.sub(f"func {CANONICAL_TEST}(t *testing.T) {{", suite, count=1)
    # The suite arrives as bare test functions with no package clause or imports
    # (they live in the prompt half upstream), so supply them. `fmt` and `testing`
    # are what every one of the 528 suites uses, checked rather than assumed.
    suite = 'package solution\n\nimport (\n\t"fmt"\n\t"testing"\n)\n\n' + suite.strip() + "\n"

    return {
        "id": problem_id(row["name"]),
        "problem": statement(dm.group(1), fn_snake, fn, sig),
        "suite": suite,
        "fn": fn,
        "sig": sig,
        "ret": ret.strip(),
    }


def parses_as_go(src: str) -> bool:
    """Does this suite parse as Go at all?

    Two of the 528 upstream suites do not, and the cause is in the published data
    rather than in this script: `mbpp_725_extract_quotation` and
    `mbpp_563_extract_values` emit Go string literals containing unescaped double
    quotes (`candidate("Cortex "A53" Based"...)`). A problem whose canonical suite
    cannot compile has no oracle, so it must be dropped and *counted* — shipping it
    would put a permanently-red directory in the tree and, worse, hand the estimator
    a problem it can never label, which reads as a model failure rather than a
    benchmark defect.

    Uses `gofmt -e` (parse and report errors) rather than a hand-rolled check, so
    the arbiter of "is this Go" is the Go toolchain.
    """
    if shutil.which("gofmt") is None:
        return True  # cannot tell; the caller warns about the missing toolchain
    res = subprocess.run(["gofmt", "-e"], input=src, capture_output=True, text=True, check=False)
    return res.returncode == 0


def contradiction(suite: str) -> str:
    """Does the suite demand two different answers for the same arguments? Returns ""
    or a description of the contradiction.

    A third gate, and unlike the first two it is about *semantics*: such a suite
    type-checks perfectly and parses fine, but no implementation can satisfy it,
    because a function is a function. Exactly one of the 489 problems that clear the
    other two gates fails this one — `he_92_any_int`, whose table asserts both
    `candidate(3, 4, 7) -> true` and `candidate(3.0, 4, 7) -> false`. Upstream the
    distinction is real (`isinstance(3.0, int)` is False in Python); through Go's
    `func AnyInt(x float64, y float64, z float64) bool` the two calls are *the same
    call*, so the transpilation destroyed the only thing the problem was testing.

    Numeric literals are compared by **value**, not by spelling: `3` and `3.0` must
    normalise to the same argument, since that is precisely the collision that makes
    the suite unsatisfiable. This is sound only because the signature has already
    type-checked against the suite, so a literal's spelling carries no type
    information the parameter list does not already fix.

    One member is not a reason to skip the gate. It costs one pass over text already
    in memory, and the failure it prevents is the expensive kind: a problem no
    implementation can pass, silently scored as a model failure. It is deliberately
    conservative — it reports only exact-duplicate argument lists with differing
    expectations, never a *suspected* inconsistency. `mbpp_802_count_rotation`, whose
    expectations follow no rule (`[3,2,1] -> 1` and `[1,3,2] -> 2`, though neither
    list can be rotated into sorted order at all), passes this gate and should: its
    arguments are all distinct, so a lookup table satisfies it. Unsatisfiable and
    merely-wrong are different defects, and only the first is decidable here.
    """
    seen: dict[str, str] = {}
    for args, exp in CALL_RE.findall(suite):
        a, e = _norm_literals(args), _norm_literals(exp)
        if a in seen and seen[a] != e:
            return f"candidate({a[:60]}) expected both {seen[a][:30]} and {e[:30]}"
        seen[a] = e
    return ""


def typechecks(d: pathlib.Path, sig: str) -> str:
    """Does the suite type-check against the pinned signature? Returns "" or the error.

    `parses_as_go` only asks whether the suite is *syntactically* Go. 37 of the 526
    that clear it do not *type-check*, and every one is an upstream defect that would
    otherwise be paid for twice — once in stage-2 tokens spent generating a reference
    that can never compile, and again as a permanently-red directory that reads as
    model failure rather than benchmark defect. All 37 come from MultiPL-E's
    Python-to-Go transpiler, in three shapes:

      * **Heterogeneous arguments** (12). The prompt declares `[]interface{}` but the
        suite passes `[]int{...}` in one case and `[]string{...}` in the next
        (`mbpp_390_add_string`). Go has no implicit conversion to `[]interface{}`, so
        *no* signature satisfies both cases — this is not a signature-extraction bug
        and cannot be fixed by extracting the type from the tests instead, because the
        tests disagree with each other.
      * **Internally invalid literals** (23). `[][]int{[]interface{}{3, 4}}`
        (`mbpp_400_extract_freq`): the outer and inner element types contradict each
        other, so the literal is invalid whatever the function's signature is.
      * **Two one-offs.** `mbpp_105_count` contains a literal `UNKNOWN` type (the
        transpiler's placeholder for an inference failure), and `mbpp_67_bell_number`
        expects a 55-digit constant that overflows `int`.

    `go vet`, not `go build`: build does not compile `_test.go` files, so a
    signature/suite mismatch clears build and only fails later at test-binary link.
    That is the same reason vet is a rung on the verifier ladder (see CLAUDE.md).

    The stub body is `panic(...)`, which type-checks for *any* return type and so
    needs no zero-value table — one less thing to get wrong for the 53 problems
    returning `[]interface{}`.
    """
    stub = d / "stub.go"
    stub.write_text(f"package solution\n\n{sig} {{\n\tpanic(\"typecheck stub\")\n}}\n")
    try:
        res = subprocess.run(
            ["go", "vet", "./..."], cwd=d, capture_output=True, text=True, check=False, timeout=120
        )
        if res.returncode == 0:
            return ""
        # Last line is the actionable one; the preceding lines are package banners.
        out = (res.stderr or res.stdout).strip().splitlines()
        return out[-1].strip() if out else "go vet failed with no output"
    except subprocess.TimeoutExpired:
        return "go vet timed out"
    finally:
        stub.unlink(missing_ok=True)


def gofmt(paths: list[pathlib.Path]) -> str:
    """Format the emitted Go files in place.

    Not cosmetic: `make check` and CI both run `gofmt -l .` over the whole tree,
    so an unformatted generated file fails the build for every unrelated change.
    Upstream's suites are indented with a mix of two and three spaces, so they are
    never gofmt-clean as shipped. Returns a message if gofmt is unavailable, so a
    machine without a Go toolchain still gets a usable benchmark and a warning
    rather than a silent tree-wide CI failure later.
    """
    if shutil.which("gofmt") is None:
        return "gofmt not on PATH: emitted .go files are NOT formatted and will fail `make check`"
    # One invocation for everything: gofmt startup dominates per-file cost.
    res = subprocess.run(
        ["gofmt", "-w", *[str(p) for p in paths]],
        capture_output=True,
        text=True,
        check=False,
    )
    if res.returncode != 0:
        return f"gofmt failed: {res.stderr.strip()}"
    return ""


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--parquet", nargs="+", required=True, help="MultiPL-E Go parquet files")
    ap.add_argument("--out", required=True, help="output benchmark directory")
    ap.add_argument("--limit", type=int, default=0, help="ingest at most N problems (smoke tests)")
    ap.add_argument("--jobs", type=int, default=8, help="parallel go vet invocations for the type-check gate")
    args = ap.parse_args()

    try:
        import pyarrow.parquet as pq
    except ImportError:
        print("needs pyarrow: pip install pyarrow", file=sys.stderr)
        return 2

    rows = []
    for f in args.parquet:
        rows += pq.read_table(f).to_pylist()

    out = pathlib.Path(args.out)
    (out / "refs").mkdir(parents=True, exist_ok=True)

    ingested, skipped, unparseable, contradictory = [], [], [], []
    for row in rows:
        got = ingest_row(row)
        if got is None:
            skipped.append(row["name"])
            continue
        if not parses_as_go(got["suite"]):
            unparseable.append(got["id"])
            continue
        # Free text check, so it runs before the subprocess gates below.
        if c := contradiction(got["suite"]):
            contradictory.append((got["id"], c))
            continue
        ingested.append(got)
        if args.limit and len(ingested) >= args.limit:
            break

    # Ingest is deterministic and sorted, so a re-run produces a byte-identical
    # benchmark and diffs stay reviewable.
    ingested.sort(key=lambda r: r["id"])

    seen = {}
    for r in ingested:
        if r["id"] in seen:
            print(f"duplicate id {r['id']}", file=sys.stderr)
            return 1
        seen[r["id"]] = True

    # Write the per-problem directories first, because the type-check gate needs a
    # real module on disk to run `go vet` in. Anything it rejects is removed again
    # below, before problems.jsonl and manifest.json are written — so those two
    # files never list a problem whose suite cannot compile.
    written = []
    for r in ingested:
        d = out / "refs" / r["id"]
        d.mkdir(parents=True, exist_ok=True)
        (d / "go.mod").write_text(f"module bench/{r['id']}\n\ngo 1.26\n")
        p = d / "solution_test.go"
        p.write_text(r["suite"])
        written.append(p)

    # gofmt before type-checking, not after: a rejected problem's directory is
    # deleted, and formatting a file that is about to be removed wastes the work.
    warn = gofmt(written)

    untyped = []
    if shutil.which("go") is None:
        print(
            "WARNING: go not on PATH — skipping the type-check gate. Expect ~37 of "
            "526 problems to have suites that cannot compile; re-run with a Go "
            "toolchain before spending anything on stage 2.",
            file=sys.stderr,
        )
    else:
        with cf.ThreadPoolExecutor(max_workers=args.jobs) as ex:
            errs = list(ex.map(lambda r: typechecks(out / "refs" / r["id"], r["sig"]), ingested))
        untyped = [(r["id"], e) for r, e in zip(ingested, errs) if e]
        bad = {i for i, _ in untyped}
        for i in bad:
            shutil.rmtree(out / "refs" / i, ignore_errors=True)
        ingested = [r for r in ingested if r["id"] not in bad]

    with (out / "problems.jsonl").open("w") as fh:
        for r in ingested:
            fh.write(json.dumps({"id": r["id"], "problem": r["problem"]}, ensure_ascii=False) + "\n")

    # The signature manifest is what stage 2 (reference generation) and any
    # re-ingest consume; it also documents the exported-name divergence per problem.
    with (out / "manifest.json").open("w") as fh:
        json.dump(
            [{"id": r["id"], "fn": r["fn"], "sig": r["sig"], "ret": r["ret"]} for r in ingested],
            fh,
            indent=2,
        )
        fh.write("\n")

    print(f"ingested {len(ingested)} of {len(rows)} problems -> {out}")
    if skipped:
        print(f"skipped {len(skipped)} (unexpected prompt shape): {', '.join(skipped[:8])}")
    if unparseable:
        # Named in full, not truncated: these are the benchmark's known holes and a
        # reader comparing n against the published 528 needs to see all of them.
        print(f"skipped {len(unparseable)} (upstream suite does not parse as Go): {', '.join(unparseable)}")
    if contradictory:
        print(f"skipped {len(contradictory)} (upstream suite is unsatisfiable):")
        for i, c in contradictory:
            print(f"    {i}: {c}")
    if untyped:
        print(f"skipped {len(untyped)} (upstream suite does not type-check):")
        for i, e in untyped:
            print(f"    {i}: {e}")
    if warn:
        print(f"WARNING: {warn}", file=sys.stderr)
    print("no solution.go written yet — run stage2_references.py to generate and validate them")
    return 0


if __name__ == "__main__":
    sys.exit(main())
