#!/usr/bin/env python3
"""Stage 2 of MultiPL-E ingestion: produce an execution-validated reference
solution for each problem.

MultiPL-E ships prompts and tests but no solutions, and go-cascade needs one per
problem for two independent reasons:

  * `calibrate -refs <dir>` runs each problem's reference through the *generated*
    test suite. A refuted reference flags the record `OracleUnsound` and excludes it
    from calibration (invariant #4). Without references, the spec model's test bugs
    silently inflate measured risk — observed live, not hypothetical.
  * the §3.7 estimator uses each problem's canonical suite as an independent oracle,
    and pins the API extracted from the reference so candidates compile against it.

So the reference is not decoration; a wrong one corrupts two load-bearing gates.

**What makes a reference trustworthy here.** It is written by a frontier model, but
it is *accepted* only if it passes MultiPL-E's own human-derived test suite by
execution (`go test`, the whole suite, no filter). The suite is the same artifact
every other paper using this benchmark scores against, and it was not written by the
model. So the honest description is: **model-written, human-test-validated** — not
"human-authored". That distinction is real and belongs in any write-up that uses
this benchmark, because a defect the upstream suite does not catch will pass into
the reference unnoticed. It bounds the reference's quality by the upstream suite's
strength, which is exactly the same bound the published pass@k numbers carry.

A problem whose reference cannot be validated within `--attempts` tries is **left
without a solution.go** and reported. It is better to run the §5.5 experiment at
n=480 with sound references than at n=526 with 46 quiet lies: `loadReferences`
already tolerates a missing reference per problem, and the id simply carries no
oracle-soundness gate.

Cost: one to a few frontier calls per problem. At 526 problems this is the expensive
step of ingestion, so it checkpoints after every problem and `--resume` skips any id
that already has a validated solution.go. Kill it freely.
"""

import argparse
import json
import pathlib
import re
import subprocess
import sys
import time

SYSTEM = """You write correct, idiomatic Go. You are given a problem statement and the
exact signature to implement. Reply with ONE ```go fenced code block and nothing
else: `package solution`, the standard-library imports you need, and the function.
No tests, no main, no commentary, no unexported helper duplicating the function name.
Correctness on edge cases matters more than brevity."""

FENCE_RE = re.compile(r"```(?:go)?\s*\n(.*?)```", re.S)


def extract_code(reply: str) -> str | None:
    """Pull the Go source out of a reply. Mirrors prompt.ExtractCode's tolerance:
    a model that omits the fence but emits a package clause is still usable."""
    if m := FENCE_RE.search(reply):
        return m.group(1).strip()
    if "package " in reply:
        return reply.strip()
    return None


def validate(refdir: pathlib.Path, src: str, timeout: int) -> tuple[bool, str]:
    """Write src as solution.go and run the WHOLE canonical suite against it.

    No `-run` filter, deliberately: filtering is precisely the bug that made the
    first estimator run measure against 40% of its oracle. `go vet` runs too,
    because `go build` does not compile _test.go files, so a signature mismatch
    against the suite would otherwise survive the build (a documented gotcha of this
    repo's verifier ladder, and the reason vet is a rung of it).

    On failure the solution.go is REMOVED, so a half-validated reference can never
    be picked up by loadReferences on a later run.
    """
    sol = refdir / "solution.go"
    sol.write_text(src if src.endswith("\n") else src + "\n")
    try:
        for args in (["go", "vet", "./..."], ["go", "test", "-count=1", "./..."]):
            res = subprocess.run(
                args, cwd=refdir, capture_output=True, text=True, timeout=timeout, check=False
            )
            if res.returncode != 0:
                sol.unlink(missing_ok=True)
                out = (res.stdout + res.stderr).strip()
                return False, f"{args[1]}: {out[:600]}"
        return True, ""
    except subprocess.TimeoutExpired:
        sol.unlink(missing_ok=True)
        return False, f"timed out after {timeout}s (likely an infinite loop)"


def already_valid(refdir: pathlib.Path) -> bool:
    """A reference counts as done only if it exists AND still passes. Cheap enough
    (a warm build cache makes this ~100ms) and it means --resume cannot inherit a
    stale reference from an earlier, differently-ingested tree."""
    if not (refdir / "solution.go").exists():
        return False
    ok, _ = validate(refdir, (refdir / "solution.go").read_text(), 60)
    return ok


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--bench", required=True, help="benchmark dir produced by ingest.py")
    ap.add_argument(
        "--model",
        default="us.anthropic.claude-opus-4-5-20251101-v1:0",
        help="Bedrock model id used to write the references",
    )
    ap.add_argument("--region", default="us-west-2")
    ap.add_argument("--attempts", type=int, default=3, help="generation attempts per problem")
    ap.add_argument("--timeout", type=int, default=60, help="per-command seconds")
    ap.add_argument("--limit", type=int, default=0, help="process at most N problems")
    ap.add_argument("--resume", action="store_true", help="skip problems with a passing solution.go")
    ap.add_argument("--dry-run", action="store_true", help="report what would be generated; no spend")
    args = ap.parse_args()

    bench = pathlib.Path(args.bench)
    manifest = json.loads((bench / "manifest.json").read_text())
    problems = {json.loads(l)["id"]: json.loads(l)["problem"] for l in (bench / "problems.jsonl").open()}

    todo = []
    for m in manifest:
        refdir = bench / "refs" / m["id"]
        if args.resume and already_valid(refdir):
            continue
        todo.append(m)
        if args.limit and len(todo) >= args.limit:
            break

    print(f"{len(todo)} of {len(manifest)} problems need a reference", file=sys.stderr)
    if args.dry_run:
        # Deliberately prints the count and exits: this is the step that costs money,
        # so it must be possible to see the size of the bill before agreeing to it.
        print(f"dry run: would make up to {len(todo)} x {args.attempts} calls to {args.model}")
        return 0
    if not todo:
        return 0

    import boto3

    rt = boto3.client("bedrock-runtime", region_name=args.region)

    report_path = bench / "references.json"
    report = {}
    if report_path.exists():
        report = json.loads(report_path.read_text())

    failed = []
    for i, m in enumerate(todo, 1):
        pid = m["id"]
        refdir = bench / "refs" / pid
        user = f"{problems[pid]}\n\nThe function MUST be exactly:\n    {m['sig']}\n"
        ok, diag, attempts_used = False, "", 0
        for attempt in range(1, args.attempts + 1):
            attempts_used = attempt
            msg = [{"role": "user", "content": [{"text": user}]}]
            if diag:
                # Feed the execution failure back. This is a *reference*, not a
                # cascade candidate, so there is no holdout to protect and no
                # invariant #1 concern: the canonical suite is the thing the
                # reference must satisfy, and it never becomes a routing oracle.
                msg.append({"role": "assistant", "content": [{"text": "(previous attempt)"}]})
                msg.append(
                    {
                        "role": "user",
                        "content": [
                            {"text": f"That failed:\n{diag}\n\nReply with the corrected full file."}
                        ],
                    }
                )
            try:
                resp = rt.converse(
                    modelId=args.model,
                    system=[{"text": SYSTEM}],
                    messages=msg,
                    inferenceConfig={"maxTokens": 4000, "temperature": 0.2},
                )
            except Exception as e:  # noqa: BLE001 — any API error is retried, then reported
                diag = f"bedrock: {e}"
                time.sleep(2 * attempt)
                continue
            reply = "".join(c.get("text", "") for c in resp["output"]["message"]["content"])
            src = extract_code(reply)
            if src is None:
                diag = "reply contained no Go code block"
                continue
            ok, diag = validate(refdir, src, args.timeout)
            if ok:
                break

        report[pid] = {"validated": ok, "attempts": attempts_used, "model": args.model}
        if not ok:
            report[pid]["diag"] = diag[:600]
            failed.append(pid)
        # Checkpoint every problem: this step is long and gets killed.
        report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
        mark = "ok " if ok else "FAIL"
        print(f"[{i}/{len(todo)}] {mark} {pid} (attempt {attempts_used})", file=sys.stderr)

    validated = sum(1 for v in report.values() if v["validated"])
    print(f"\nvalidated {validated} references; {len(failed)} still unvalidated")
    if failed:
        print(f"no reference for: {', '.join(failed[:20])}")
        print("these problems keep NO solution.go — they carry no oracle-soundness gate")
    print(f"report: {report_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
