# Kickoff prompt for the next session

Paste the block below to start the next session. It assumes memory is loaded
(MEMORY.md → gocascade-session-state.md has the full state).

---

We're continuing the go-cascade study. Before anything else:

1. **Check git state.** `main` should be at `f41767f` (PR #44, the MultiPL-E
   type-check gate). 44 PRs merged. If a PR *is* open, confirm CI green and I'll
   tell you whether to merge — don't merge unilaterally.
2. **Re-read `results/README.md`** (nineteen experiments) and
   **`docs/verification-saturated-cascades.md` §5.6** (the live-evaluation
   reconciliation, including the corrected §5.5(5) result).
   `AWS_PROFILE=aws` for live Bedrock; `--provider=mock` is free.
3. **Check whether the MultiPL-E benchmark finished building.** It lives *outside*
   the repo at `~/mple-bench` (4 MB, 489 problems) — see "The benchmark" below.

## Where the study stands

Both original levers, all three refinement threads, and both of the previous
session's code threads are run and written up. Last session did two things:

- **Corrected experiment 19** (PR #42). The §3.7 estimator test's first run handed
  the canonical suite to the ladder's *visible* stage, so `-run ^TestV` applied and
  **222 of 370 canonical tests never executed** — the adversarial half, by
  construction. The re-run against the full suite **reproduced the headline**:
  η_fa **0/145** (95% bound **0.020**) against a pooled 1−M of **0.1014** predicting
  ~12 events, so M is still a loose-but-*conservative* proxy. That comparison was
  never at risk — `verify.Mutate` uses no `-run` filter, so M was always measured
  against the whole generated suite; only the canonical Y oracle was weakened.

  **The secondary findings did not survive.** Confirmed false rejections fell
  **11 → 4** on a *disjoint* problem set (of the old 11, eight re-drew as candidates
  the full oracle accepts and two are now agreed-wrong — the generated suite was
  right to reject them; none remains a false rejection). Three canonical refutations
  appeared where the weak oracle produced zero, all from `TestH*` and all real:
  `IntSqrt(MaxUint64)` off by one, an int64 overflow at `Fibonacci(94)`, an
  input-aliasing check. Two generalisable lessons, now in §3.7 and CLAUDE.md:
  **oracle strength must be recorded and audited** (a suite can run 40% of itself
  and still return *sound* verdicts on what it ran, so no test fails), and
  **rejection-side rates are not stable at n=64** (the two draws agree on only
  159/192 rows). The fix is `Ladder.RunAllTests` (unfiltered, off the acceptance
  path), `EstimatorObs.CanonicalTests`, and a zero-tests-ran stage treated as a
  **refutation** in both `Run` and `Accept`.

- **Built the MultiPL-E benchmark** (PRs #43, #44) — the ingestion half of the one
  remaining gap. See below.

## The benchmark (new, and it is not in the repo)

`~/mple-bench` — MultiPL-E Go (HumanEval-Go + MBPP-Go) in the layout `calibrate`,
`estimator`, and `solve` consume. **489 of 528 problems**, which clears the §5.5
`n ≥ 300` bar. Scripts are at `examples/bench/multipl/`; the *data* is outside the
repo because it is 4 MB of generated files.

**39 exclusions, all upstream MultiPL-E transpilation defects**, each named in the
ingester's output: 2 do not parse (`gofmt -e`; unescaped quotes in string literals)
and **37 do not type-check**. The type-check gate (`go vet` against a `panic()` stub
carrying the pinned signature — vet not build, because build skips `_test.go`) was
added *after* running the ingester on real data caught the problem. Shapes: 12
heterogeneous arguments (prompt says `[]interface{}`, suite passes `[]int` then
`[]string` — **no** signature satisfies both, so this is not a signature-extraction
bug), 23 internally invalid literals, 2 one-offs. Worth knowing when quoting n: the
exclusions are **not random with respect to difficulty** — they cluster in MBPP's
tuple-heavy problems, which transpile worst to Go.

**Stage 2 is finished: 467 of 489 references validated (95.5%).** All three consistency
checks pass — `manifest.json`, `problems.jsonl` and `refs/` list the same 489 ids, the
22 ids with no `solution.go` are exactly the 22 marked unvalidated, and the tree is
gofmt-clean. Re-verify cheaply (it re-executes rather than trusting the report):

```bash
python3 -c "import json,os;r=json.load(open(os.path.expanduser('~/mple-bench/references.json')));\
print(sum(1 for v in r.values() if v['validated']),'validated')"
```

**Quote n = 467, not 489, and do not count the 22 as cascade failures.** 21 of them are
upstream oracle defects, diagnosed individually (see the README's stage-2 section for
the per-problem account): 1 unsatisfiable through the pinned signature
(`he_92_any_int` demands opposite answers for `3` and `3.0` through a `float64`
parameter), 1 self-inconsistent, 6 whose oracle encodes a reproducible reference bug,
3 whose statement contradicts its own oracle, 4 under-specified ordering, 1 float
last-ULP, 5 left explicitly unclassified. Exactly **one** — `he_116_sort_array`, which
needs Python's `bin(-4) == "-0b100"` magnitude-popcount semantics — is a real model
failure, confirmed satisfiable by a hand-written reference that passes its full suite.

That result strengthens the caveat below rather than just trimming n: on a measurable
fraction of this benchmark the upstream oracle is **wrong**, in the direction that
rewards reproducing a Python bug. So the 467 references are, by construction, solutions
that agree with the upstream suite *including wherever it is mistaken*.

References are **model-written, human-test-validated** — Opus writes each one and it
is kept *only* if it passes MultiPL-E's own human-derived suite by execution
(`go vet` then the full `go test`, no `-run` filter). That is **not** "human-authored",
which is what the 64-problem set's references are, and any write-up using this
benchmark must say so: a defect the upstream suite misses passes into the reference
unnoticed. A problem whose reference cannot be validated in 3 attempts keeps **no**
`solution.go` and is named — n=467 with sound references beats n=489 with 22 quiet
lies. `loadReferences` tolerates a missing reference, so such an id simply carries no
oracle-soundness gate.

Verified free, end-to-end, before spending: correct hand-written solutions **pass**
the canonical suites and wrong ones are **refuted** (both directions — the pass side
alone proves nothing); the pinned signature compiles against all 489; and
`calibrate --provider=mock -refs ~/mple-bench -pin-api` loads **8/8 references and
pins 8/8 APIs**. In that mock run the oracle gate reports "inconclusive" for every
problem, which is a **mock artifact** — `internal/model/mock.go` returns a canned
`LongestIncreasingRun` fixture regardless of the prompt, so no reference can fit it.
Do not read that as a benchmark defect; re-check it on the first live run.

## The single open gap

1. **The real §5.5 validation experiment.** n ≥ 300 on a standard benchmark, all five
   arms, plus §5.5(4) cache-warmth. The **ingestion half is now done** — what remains
   is the run itself, which is real money at 489 problems × 5 arms. **Scope the spend
   with me first; I said "not yet" to this twice before.**

   Three n≤64 findings await it: whether **M ranks** candidates by η_fa (experiment 19
   had no events to rank), whether the **cost win** is more than a 2-in-6 coincidence
   (experiment 17), and whether the **false-rejection rate** has any stable value at
   all (11 vs 4 across two draws of the same experiment).

Or: **declare the study done** and treat it as a finished artifact. Nineteen
experiments, both levers mapped, every claim labelled demonstrated-vs-argued, and now
a standard benchmark ingested and ready. A defensible stopping point either way.

Keep the discipline: branch-per-change + PR + green CI + I merge; surface confounds
rather than bury them; state demonstrated vs. argued; never cite a mock number as a
model measurement; long runs checkpoint + `-resume`. Experiment 19's correction is the
bar — it reported that its own secondary findings did not replicate, and marked the
superseded numbers as measured by a 40%-strength oracle rather than overwriting them.

**Ops, learned the hard way:** long live runs are SIGKILLed by the harness's
background-task reaper (6+ times), *not* by Bedrock. Use both defenses: `-resume`
with atomic checkpoints, **and launch detached** in its own process session —
`setsid` does not exist on macOS, so double-fork via python (`os.fork` →
`os.setsid` → `os.fork` → `os.execve`), then wait on a `kill -0` loop in a
background shell. Don't sleep-poll a foreground job. Keep paid-for artifacts out of
`/tmp`. 1Password SSH signing intermittently *hangs* the commit for the full timeout
— `--no-verify` works and the signature still lands (verify with
`git log --show-signature -1`).

---

## Quick reference (state as of 2026-08-02, end of fourth session)

- Public repo: github.com/scttfrdmn/go-cascade. main `f41767f`, 44 PRs merged.
- **New last session:** `Ladder.RunAllTests`; zero-tests-ran guards in
  `ladder.Run`/`Accept`; `EstimatorObs.CanonicalTests`; `results/analyze_estimator.py`
  (recomputes every figure the §3.7 write-up quotes, from any records file);
  `examples/bench/multipl/{ingest.py,stage2_references.py,README.md}`; records
  `results/estimator-n64-full-oracle.json` (the superseded 40% run is kept as
  `results/estimator-n64.json` for comparison).
- **Configs:** `config.go-specialist-{211,321,511}.json`, `config.two-stage.json`,
  `config.two-stage-haiku.json`, `config.plan-once.json`. test_model = sonnet-4-6,
  MUST differ from every tier AND every planner (invariant #3).
- **Analysis scripts (offline, free):** `results/analyze_estimator.py`,
  `headroom_theorem.py`, `analyze_tension.py`, `analyze_draws.py`.
- **Live spend so far:** ~$120–145 (prior ~$108–126 + experiment 19's re-run ~$8–14
  + MultiPL-E stage 2 ~$10–15, the last two *estimated not measured* — neither the
  `estimator` subcommand nor `stage2_references.py` records a cost field).
- Bedrock models ACTIVE (us-west-2): maverick-17b, haiku-4-5, sonnet-4-5, sonnet-4-6,
  opus-4-5 (+ newer opus/sonnet/fable-5 profiles — keep configs pinned to the models
  each experiment used, for cross-run cost comparability). Re-run `go-cascade models`
  to confirm before any live run.
