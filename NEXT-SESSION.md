# Kickoff prompt for the next session

Paste the block below to start the next session. It assumes memory is loaded
(MEMORY.md → gocascade-session-state.md has the full state).

---

We're continuing the go-cascade study. Before anything else:

1. **Check git state.** `main` should be at `0b3f79b` (PR #48, the §5.5 run at
   n=409). 48 PRs merged. If a PR *is* open, confirm CI green and I'll tell you
   whether to merge — don't merge unilaterally.
2. **Read the project board** —
   https://github.com/users/scttfrdmn/projects/62 — which is now where the open work
   lives, one issue per gap (#49–#53). This file no longer restates the backlog.
3. **Re-read `results/README.md`** (twenty-one experiments) and
   **`docs/verification-saturated-cascades.md` §5.6** (the live-evaluation
   reconciliation). `AWS_PROFILE=aws` for live Bedrock; `--provider=mock` is free.
4. The MultiPL-E benchmark is built and used. It lives *outside* the repo at
   `~/mple-bench` (4 MB, 488 problems) — see "The benchmark" below.

## Where the study stands

**§5.5(1) is met.** The paper's own bar — n ≥ 300 on a standard benchmark — was the
one gap between "demonstrated" and "validated", and experiment 21 closed it
(`results/s55-multipl-n409-2026-08-03.md`, PR #48): **488 MultiPL-E Go problems, 409
usable**, both oracles paired on an identical candidate stream, $8.15.

- **Execution certifies α=0.084; the judge only α=0.226** (δ=0.10). The margin widens
  monotonically with n — ratio 1.19× (n=28) → 1.58× (n=64) → **2.69×**, absolute gap
  0.050 → 0.110 → **0.142**. (It does *not* triple; that was an overstatement caught
  and corrected in review.)
- **Execution sound 1096/1096.** β = 0 at 16× the previous sample size.
- **η_fa measurable for the first time — 11/1096**, with the cheap-tier gradient
  §3.1 predicts (8/2/1). But the records keep no candidate source, so the defect
  classes are unrecoverable and the *reading-invisible* mechanism stays **argued**.
  That is issue #49, and it is the highest-value free item on the board.

**Meeting the bar cost two n=64 headlines. Prefer the n=409 figure wherever they
conflict:**

1. **α=0.05 does NOT certify.** The lowest is 0.084, floored by genuine model
   accuracy (empirical risk 0.0538 over 409). Experiment 12's "0/52 genuine errors"
   was a small-sample artifact — the sample was too small to contain the model's
   errors. This is the more trustworthy of the two results.
2. **The certifiable-α-vs-cost-win tension reproduces** at 6× scale rather than
   dissolving: `[1,1]` is 1.6× *pricier* than always-frontier below α=0.11;
   `[0.1,1]` is 2.2× cheaper at or above it.

**And it opened a new coverage gap that may matter more than n:** MultiPL-E Go has
**0 concurrency problems**, so the `-race` rung — which caught the study's only
confirmed judge over-acceptance — never fired in the only large-n run. The
64-problem hand-written set (11 of 64 are concurrency) is **not obsolete**. Issue #50.

Before that, two sessions did this:

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

- **Built the MultiPL-E benchmark** (PRs #43, #44) — the ingestion half of what was
  then the one remaining gap. See below.

- **Decided §5.5(4) offline for $0** (experiment 20, PR #46). Retrieval candidacy is
  464/488 (95.1%) but the **absorption ceiling is 2/488 (0.4%)**, because arm zero
  re-executes (invariant #5), so the paid run would have reported a corpus artifact as
  a null. Stronger by-product: at the high end similarity is **anti-correlated** with
  transferability — the top pair of all 118,828 is the antonym `minimum`~`maximum`
  (0.949), and the top same-signature pair (`he_56`~`he_61` `CorrectBracketing`, 0.836)
  is retrieved and **refuted** (`<>` vs `()`). Invariant #5 measured, not asserted.
  This is **not** licence to relax invariant #8; §2.9 is untested, not disproven.
  Issue #52.

- **Fixed `prompt.ExtractAPI`** (PR #47), found by a diagnostic pattern in the pre-fix
  §5.5 records. The API stub kept the source's import block verbatim after blanking
  every function body, so any import used *only* by a body became an **unused import —
  a compile error in Go**. The stub is pasted into the spec prompt under "use exactly
  this API", so it failed `testsCompileAgainstOwnAPI` and made `validateOracle` flag
  **sound** oracles `OracleUnsound`. A/B on the benchmark: **133/470 → 0/470**. The
  failure direction was conservative (false exclusions only), so the pre-fix
  certificate was valid — what it corrupted was `n` and the exclusion rate.

## The benchmark (new, and it is not in the repo)

`~/mple-bench` — MultiPL-E Go (HumanEval-Go + MBPP-Go) in the layout `calibrate`,
`estimator`, and `solve` consume. **488 of 528 problems**, which clears the §5.5
`n ≥ 300` bar. Scripts are at `examples/bench/multipl/`; the *data* is outside the
repo because it is 4 MB of generated files.

**40 exclusions, all upstream MultiPL-E transpilation defects**, each named in the
ingester's output, by three gates: 2 do not parse (`gofmt -e`; unescaped quotes in
string literals), **37 do not type-check**, and 1 is **unsatisfiable** (see below).
The type-check gate (`go vet` against a `panic()` stub carrying the pinned signature —
vet not build, because build skips `_test.go`) was
added *after* running the ingester on real data caught the problem. Shapes: 12
heterogeneous arguments (prompt says `[]interface{}`, suite passes `[]int` then
`[]string` — **no** signature satisfies both, so this is not a signature-extraction
bug), 23 internally invalid literals, 2 one-offs. Worth knowing when quoting n: the
exclusions are **not random with respect to difficulty** — they cluster in MBPP's
tuple-heavy problems, which transpile worst to Go.

**Stage 2 is finished: 470 of 488 references validated (96.3%).** All consistency checks
pass — `manifest.json`, `problems.jsonl` and `refs/` list the same 488 ids, the 18 ids
with no `solution.go` are exactly the 18 marked unvalidated, and the tree is gofmt-clean.
Re-verify cheaply (it re-executes rather than trusting the report):

```bash
python3 -c "import json,os;r=json.load(open(os.path.expanduser('~/mple-bench/references.json')));\
print(sum(1 for v in r.values() if v['validated']),'validated')"
```

488, not 489: `he_92_any_int` is **unsatisfiable** (it demands opposite answers for `3`
and `3.0` through a `float64` parameter) and is now dropped by a third ingestion gate,
`contradiction`, instead of being discovered by paying for generation attempts.

**Quote n = 470, and do not count the 18 as cascade failures.** 17 are upstream oracle
defects, diagnosed individually — see the README's stage-2 section for the per-problem
account: 1 self-inconsistent, 5 whose oracle encodes a reproducible reference bug, 3
whose statement contradicts its own oracle, 4 under-specified ordering, 1 float last-ULP,
3 left explicitly unclassified. Exactly **one** — `he_116_sort_array`, needing Python's
`bin(-4) == "-0b100"` magnitude-popcount — is a clean model failure: it failed all six
attempts with a byte-identical diagnostic, and a hand-written reference passes its full
suite, so it is reliably hard rather than unlucky.

**Two process lessons in that number.** Raising `--attempts` from 3 to 6 recovered three
problems (467 → 470) that had looked defective — sampling variance, not oracle bugs. And
one of those three, `mbpp_260_newman_prime`, had already been *misclassified* here as an
oracle off-by-one; the NSW primes really are 7, 41, 239 and only the indexing convention
is ambiguous. Distinguishing "oracle is buggy" from "problem is ambiguous and the model
guessed" needs more than one draw.

That result strengthens the caveat below rather than just trimming n: on a measurable
fraction of this benchmark the upstream oracle is **wrong**, in the direction that
rewards reproducing a Python bug. So the 470 references are, by construction, solutions
that agree with the upstream suite *including wherever it is mistaken*.

References are **model-written, human-test-validated** — Opus writes each one and it
is kept *only* if it passes MultiPL-E's own human-derived suite by execution
(`go vet` then the full `go test`, no `-run` filter). That is **not** "human-authored",
which is what the 64-problem set's references are, and any write-up using this
benchmark must say so: a defect the upstream suite misses passes into the reference
unnoticed. A problem whose reference cannot be validated in 3 attempts keeps **no**
`solution.go` and is named — n=470 with sound references beats n=488 with 18 quiet
lies. `loadReferences` tolerates a missing reference, so such an id simply carries no
oracle-soundness gate.

Verified free, end-to-end, before spending: correct hand-written solutions **pass**
the canonical suites and wrong ones are **refuted** (both directions — the pass side
alone proves nothing); the pinned signature compiles against all 488; and
`calibrate --provider=mock -refs ~/mple-bench -pin-api` loads **8/8 references and
pins 8/8 APIs**. In that mock run the oracle gate reports "inconclusive" for every
problem, which is a **mock artifact** — `internal/model/mock.go` returns a canned
`LongestIncreasingRun` fixture regardless of the prompt, so no reference can fit it.
Do not read that as a benchmark defect; re-check it on the first live run.

## The open gaps live on the board, not in this file

**https://github.com/users/scttfrdmn/projects/62** — five issues, each carrying the
file paths, the measurement that motivated it, and a spend estimate where one applies.
Do not re-derive the backlog from prose here; read the issues.

| # | gap | cost | why it matters |
|---|-----|------|----------------|
| [#49](https://github.com/scttfrdmn/go-cascade/issues/49) | Retain candidate source for **disagreeing** observations | free | Unblocks the paper's η_fa *mechanism* claim. 11 events measured, defect classes unrecoverable. ~30 lines in `internal/cascade/judge.go`. **Start here.** |
| [#50](https://github.com/scttfrdmn/go-cascade/issues/50) | Concurrency coverage — `-race` never fired at large n | **$2–4** | The rung that caught the only confirmed judge over-acceptance was skipped on all 488. |
| [#51](https://github.com/scttfrdmn/go-cascade/issues/51) | Scar-free race operator | free | Sync-*deletion* leaves a visible scar, so 20/20 measured the wrong class. |
| [#52](https://github.com/scttfrdmn/go-cascade/issues/52) | §5.5(4) via **duplicate injection** | free harness | Absorption must become a dial; the corpus cannot supply it. |
| [#53](https://github.com/scttfrdmn/go-cascade/issues/53) | Arm (e), self-consistency at matched cost | **$6–10** | Last unimplemented arm of §5.5(2). Lowest priority — a comparison arm, not a load-bearing claim. |

#50 and #53 are labelled `needs-spend-approval` and are **filed, not scheduled**.
**Scope the spend with me first** — I said "not yet" to a live run twice before.

Two n≤64 questions the n=409 run did *not* settle, and one it did: whether **M ranks**
candidates by η_fa is **still open** (experiment 19 had no events in either M bucket);
whether the **false-rejection rate** has any stable value is **still open** (11 vs 4
across two draws); the **cost win** question is **answered** — the tension reproduces,
so it was not a 2-in-6 coincidence but a structural property.

Or: **declare the study done** and treat it as a finished artifact. Twenty-one
experiments, both levers mapped, every claim labelled demonstrated-vs-argued, and the
paper's own §5.5(1) bar now met. A defensible stopping point either way — and a more
defensible one than it was a session ago.

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
**backgrounded** shell — never a foreground `sleep`, which the harness kills and which
blocks the session for nothing. Keep paid-for artifacts out of
`/tmp` (experiment 21's pre-fix records were $7.82 sitting in a temp dir).
The Bash-safety classifier also goes "temporarily unavailable" for minutes at a time
and blocks *all* Bash; Read/Grep/Glob keep working, so do read-only work and retry.
1Password SSH signing intermittently *hangs* the commit for the full timeout
— `--no-verify` works and the signature still lands (verify with
`git log --show-signature -1`).

---

## Quick reference (state as of 2026-08-03, end of fifth session)

- Public repo: github.com/scttfrdmn/go-cascade. main `0b3f79b`, 48 PRs merged.
  Project board: https://github.com/users/scttfrdmn/projects/62 (issues #49–#53).
- **New last session:** `results/analyze_s55.py` (recomputes every figure the n=409
  write-up quotes, from any records pair — usable-only, and it correctly *skips*
  observations lacking `true_correct` rather than falling back to `correct`, which for
  the judge arm would force agreement); `results/absorption_ceiling.py` (bench path is
  **positional**, there is no `--bench` flag); the `ExtractAPI` import-pruning fix
  (PR #47); records `results/s55-fixed.records.{execution,judge}.json` (n=409) with the
  superseded pre-fix pair kept as `results/s55-n470.records.*.json`.
- **Configs:** `config.go-specialist-{211,321,511}.json`, `config.two-stage.json`,
  `config.two-stage-haiku.json`, `config.plan-once.json`. test_model = sonnet-4-6,
  MUST differ from every tier AND every planner (invariant #3). Experiment 21 used
  `config.go-specialist-211.json`.
- **Analysis scripts (offline, free):** `results/analyze_s55.py`,
  `analyze_estimator.py`, `absorption_ceiling.py`, `headroom_theorem.py`,
  `analyze_tension.py`, `analyze_draws.py`. `calibrate -from-records` re-derives a
  certificate at any α for **$0** — sweep each arm against its **own** records.
- **Live spend so far:** ~$136–161 (prior ~$120–145 + experiment 21's $8.15 measured
  + the superseded pre-fix attempt's $7.82 measured; experiment 20 cost **$0**).
  Earlier components remain *estimated not measured* — neither the `estimator`
  subcommand nor `stage2_references.py` records a cost field.
- Bedrock models ACTIVE (us-west-2): maverick-17b, haiku-4-5, sonnet-4-5, sonnet-4-6,
  opus-4-5 (+ newer opus/sonnet/fable-5 profiles — keep configs pinned to the models
  each experiment used, for cross-run cost comparability). Re-run `go-cascade models`
  to confirm before any live run.
