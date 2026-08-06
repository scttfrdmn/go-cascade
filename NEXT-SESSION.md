# Kickoff prompt for the next session

Paste the block below to start the next session. It assumes memory is loaded
(MEMORY.md → gocascade-session-state.md has the full state).

---

We're continuing the go-cascade study. Before anything else:

1. **Check git state.** `main` should be at `1d3ebf6` (PR #71, the experiment-30
   write-up). 71 PRs merged. **Two PRs may still be open** — #73 (`-seed-kind` records
   persistence, issue #72) and #75 (Bedrock cost tagging, issue #74). If a PR is open,
   confirm CI green and I'll tell you whether to merge; don't merge unilaterally.
2. **Read the project board** —
   https://github.com/users/scttfrdmn/projects/62 — which is now where the open work
   lives, one issue per gap (#49–#53). This file no longer restates the backlog.
3. **Re-read `results/README.md`** (twenty-nine experiments; 24 live, 5 decided
   offline for $0) and
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

**The two most recent experiments were both free, and both had the same shape:**
the design check found the experiment as specified would have measured nothing.

- **Experiment 22 — §5.5(4) as a controlled dial** (PR #58, issue #52). Absorption is
  *injected* into the recorded n=409 stream, so the whole sweep replays offline for $0.
  Under a head-shaped filter the certificate goes optimistic monotonically (gap +0.0147
  → +0.1486 as ρ goes 0.2 → 0.8) and at ρ=0.6 promises α=0.10 while delivering 0.134 —
  a **violated** bound. Three traps, each of which produced a wrong answer first:
  **uniform absorption is a null** (dropping a random subset of an exchangeable sample
  leaves one), so #52's own framing would have measured nothing and reported it as
  evidence — it ships as the explicit control; `Calibrate` **prefers the shadow subset
  of whatever it is handed** and every profiled record has `Shadow: true`, which made
  the uncorrected arm silently identical to the corrected one (hence `stripShadowFlags`);
  and **shadow sampling spends sample size**, so small-ε rows are flagged `Underpowered`
  rather than dropped (a missing row reads as "not swept"). **The envelope is a
  multi-seed quantity** — selective patterns are sorts and so seed-exact, but the uniform
  control is a random draw. Over 10 seeds × 4 rates max |gap| is **0.0389**, not the
  0.0267 one sweep shows, and by the honest envelope the effect holds at **ρ ≥ 0.6, not
  ρ ≥ 0.4**. `EstimateNullEnvelope` ships *inside* the output, because a bar left to the
  reader gets compared against one draw of the control.

- **Experiment 23 — arm (e) feasibility** (PR #59, issue #53). §5.5(2)'s last
  unimplemented arm. At τ=[1,1] the $0.0101/query matched budget buys median **49 / 2 /
  1** samples at the profiled 2:1:1 fan-out — below a 3-way vote on **0.0% / 79.2% /
  99.5%** of problems. The cascade's whole spend is roughly one frontier call, so a
  frontier arm (e) at matched cost is **always-frontier relabelled**; run as §5.5(2)
  literally specifies it (tier unnamed) it would have reported a degenerate configuration
  as a null about self-consistency. `selfconsistency` therefore **refuses `-sample` on a
  ruled-out tier**. Only the cheap tier is well-posed, and there it is exactly §3.5's
  contrast: 49 votes on how the code is *written* vs 2 on what it *does*, same money. The
  paid pass is **$4.12 and unrun**.

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
| ~~[#49](https://github.com/scttfrdmn/go-cascade/issues/49)~~ | ~~Retain candidate source for **disagreeing** observations~~ | free | **DONE** (PR #55). `TierObs.DisagreementSource` + `results/classify_disagreements.py`. Forensic only — nothing on the acceptance path reads it. |
| [#50](https://github.com/scttfrdmn/go-cascade/issues/50) | Concurrency coverage — `-race` never fired at large n | **$2–4** | The rung that caught the only confirmed judge over-acceptance was skipped on all 488. |
| ~~[#51](https://github.com/scttfrdmn/go-cascade/issues/51)~~ | ~~Scar-free race operator~~ | free + ~$1.20 | **GENERATOR DONE, coverage MEASURED, and the sweep RUN — both arms NULL** (experiments 28, 29, 30). `-seed-kind=scar-free-race`: **four** operators (RWMutex downgrade, escape past an explicit `Unlock`, escape past a **deferred** `Unlock`, `defer wg.Wait()`), all validated `-race`-refuted. Reference coverage on the 11 concurrency problems: **16 sites → 10 seeds**, meeting the bar of 10 registered before experiment 28's harvest (that harvest returned 9 and declined). The paid sweep then returned **scar-free 0/9, sync-deletion 0/27**, every strictness level, Fisher p = 1.0 — see below. Loop-var capture is obsolete (Go 1.22 per-iteration vars). |
| [#52](https://github.com/scttfrdmn/go-cascade/issues/52) | §5.5(4) via **duplicate injection** | free harness | Absorption must become a dial; the corpus cannot supply it. |
| [#53](https://github.com/scttfrdmn/go-cascade/issues/53) | Arm (e), self-consistency at matched cost | **$6–10** | Last unimplemented arm of §5.5(2). Lowest priority — a comparison arm, not a load-bearing claim. |

#50 and #53 are labelled `needs-spend-approval` and are **filed, not scheduled**.
**Scope the spend with me first** — I said "not yet" to a live run twice before.

**#49 and #51 are merged, the spend decision was taken, and the sweep RAN — it returned
a null in both arms, so the η_fa mechanism claim stays ARGUED** (experiment 30,
`results/scarfree-sweep-n9.md`). Sequence, in order: the bar of ≥10 seeds was registered
before experiment 28's harvest; that harvest returned 9 and **declined**; experiment 29's
deferred-escape operator supplied the tenth, so the bar was met **without the bar
moving**; the run was then priced at **~$1.20** (the seeded path skips the cascade tier
loop, so it is cheaper than the ~$3 the decline assumed), authorized explicitly, and run.
**Result: scar-free 0/9 false acceptances from 5 problems, sync-deletion 0/27 from 8, at
strict/balanced/permissive alike. Fisher one-sided 0/9 vs 0/27 = p = 1.0.** The
comparison the sweep exists to make asks whether the scar-free rate is *above* the
deletion rate; both rates are zero, so there is no gap to test. This landed in the branch
the pre-registered arithmetic gave the larger probability — 0, 1, or 2 events all read
"cannot resolve."

**Four things outlive the verdict, and they are the reason to read the write-up:**

1. **The realized denominator is not the instrument's reach.** The bar was measured on the
   *references* (10 seeds / 7 problems); `ProfileSeeded` mutates a tier-0 **model draw**,
   which came back 9 seeds / **5** problems, and the two problem sets differ in *both*
   directions. A nearly-equal total over fewer problems is more **concentrated**, so
   several mutants share a base program, effective n is *below* 9, and the ≤0.283 null
   bound is optimistic. Flagged before launch, not after.
2. **Re-run a control in-session; do not cite one across sessions.** The sync-deletion arm
   came back **0/27 from 8**, not the **0/20 from 9** on file from 2026-07-25 — same
   operator, benchmark and config, different draw. Citing July's figure beside today's
   scar-free arm would have crossed model versions inside a two-arm comparison,
   invisibly. The critical value is ≥3 against both, so the verdict is unchanged — but
   that is luck, and it is only *checkable* because the control was re-measured.
3. **`-seed-kind` wrote no records, only stdout — now FIXED (PR #73).** It had no
   `-records` path, unlike every other `calibrate` mode: per-problem seed counts and
   **mutant sources** were unrecoverable and the run was not resumable. Cheap on a null;
   fatal on a positive event, where the only interesting follow-up is *what* the judge
   missed. `cascade.SeededRecord` now persists the per-problem seed count, each mutant's
   `Desc`/`Source`/`DataRace`/`PlainRefuted`, and the per-level verdicts, checkpointed
   after every problem, with `-resume` — a persistence path, not new instrumentation, and
   forensic only. Four properties worth not undoing: the table is **derived from the
   records** (a printed rate and a persisted one cannot drift); **zero-seed rows are
   recorded with a reason** (which problems yield nothing is a coverage fact about the
   operator set, and a sampling failure is a different null from an operator with no site);
   a **cross-arm resume is refused** (it would pool two rates into one denominator); and a
   **partial row is kept but re-run** (skipping it leaves the file short of its own
   harvested count, which reads as a smaller denominator rather than as missing data).
4. **The surviving asymmetry is bound tightness, not observed rate** — 9 seeds bound the
   scar-free class at ≤0.283 against the control's ≤0.105. Overlapping intervals are not a
   mechanism, but the class §3.1 is about is the one we can least afford to sample, and
   that is a fact about the operators' reach rather than about the judge. **Do not read
   the zeros as η_fa = 0.**

The `DataRace` requirement was verified rather than assumed — `ScarFreeRaceKilledMutants`
applies the filter on top of the shared harvest, so all 9 carried a real ThreadSanitizer
report; without it this would be a null about deterministically-wrong programs in race
clothing. The route that looked cheapest for raising n, harvesting from the tier-0 model
draws, is **measured and closed** (8 raw / 6 unique / 5 from execution-correct bases).
**Only widening the corpus can raise n further**, and that carries the tuning hazard:
authoring problems mutable *by these operators* tunes the benchmark to the instrument, so
any extension must be written as a plausible Go exercise first, checked for coverage
second, and identified as post hoc.

Two n≤64 questions the n=409 run did *not* settle, and one it did: whether **M ranks**
candidates by η_fa is **still open** (experiment 19 had no events in either M bucket);
whether the **false-rejection rate** has any stable value is **still open** (11 vs 4
across two draws); the **cost win** question is **answered** — the tension reproduces,
so it was not a 2-in-6 coincidence but a structural property.

Or: **declare the study done** and treat it as a finished artifact. Thirty
experiments, both levers mapped, every claim labelled demonstrated-vs-argued, the
paper's own §5.5(1) bar met, and the last free-standing spend decision taken and
resolved. A defensible stopping point either way — and a more defensible one than it was
a session ago, since the one open question that money could have answered has now had
money spent on it.

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

## Quick reference (state as of 2026-08-05, end of seventh session)

- Public repo: github.com/scttfrdmn/go-cascade. main `1d3ebf6`, 71 PRs merged
  (**#73 and #75 may be open** — `-seed-kind` records, and Bedrock cost tagging).
  Project board: https://github.com/users/scttfrdmn/projects/62. **#49, #51 and #52 are
  closed; #50 and #53 remain, both `needs-spend-approval`** — #50 concurrency coverage
  ($2–4), #53's paid arm (e) sampling pass ($4.12, scoped by the shipped tool). #51's
  paid follow-on (the seeded scar-free sweep) was authorized and **run** — experiment
  30, both arms null; the only route left for raising its n is widening the corpus.
  Two engineering issues filed and implemented this session: **#72** (`-seed-kind`
  wrote no records — the gap experiment 30 exposed) and **#74** (untagged Bedrock
  spend). Neither is a research gap, so neither is on the board.
- **New last session:** the four **scar-free race operators**
  (`internal/verify/mutation.go`, `-seed-kind=scar-free-race`) plus their coverage
  measurement and the **paid sweep** that used them — experiments 28/29/30,
  `results/scarfree-coverage-n11.md`, `deferred-escape-n11.md`,
  `scarfree-sweep-n9.md`, and `results/scarfree_coverage.py` for the power arithmetic.
  The gap that run exposed — **`-seed-kind` wrote stdout only**: no per-problem counts,
  no mutant sources, not resumable — is closed by **PR #73** (issue #72), `-records` /
  `-resume` via `cascade.SeededRecord`. If that PR is still open, the gap is still open.
- **From the session before:** `go-cascade absorption` (§5.5(4) dial + multi-seed null
  envelope, `internal/calibrate/absorb.go`) and `go-cascade selfconsistency` (arm (e)
  feasibility gate + sampling arm, `internal/calibrate/selfconsistency.go`,
  `internal/cascade/selfconsistency.go`, `internal/cluster/text.go`); records
  `results/absorption-n409.json` and `results/arm-e-feasibility-n409.json`. Both
  subcommands are **free with `-provider=mock`** (they replay recorded costs; mock only
  builds the router).
- **Earlier still:** `results/analyze_s55.py` (recomputes every figure the n=409
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
  `analyze_tension.py`, `analyze_draws.py`, `classify_disagreements.py` (reads the
  `disagreement_source` field added by issue #49 and dumps the program behind each
  oracle/truth disagreement for defect classification; on pre-#49 records it says so
  rather than reporting zero). `calibrate -from-records` re-derives a
  certificate at any α for **$0** — sweep each arm against its **own** records.
- **Live spend so far: ~$197 by the BILL, not the ~$40 the write-ups sum to** — see
  `results/README.md` §"A correction to every cost figure on this page". No `Record`
  stores the spec cost and the shared oracle is ~91% of real spend, so **no
  records-derived figure is the invoice**; paired *ratios* are unaffected (the term
  cancels). Experiment 30 adds ~$1.20, **priced ex ante and not yet reconciled** — Cost
  Explorer lags ~1 day. Three filters when you reconcile: query **both**
  `Amazon Bedrock Service` (Claude) and `Amazon Bedrock` (open-weight); exclude usage
  types carrying a **`-mantle-`** infix (Claude Code's own traffic); and restrict to
  `input-tokens`/`output-tokens`, because `cache-read-input-token-count` under a study
  model name is other traffic (go-cascade sends no cached prompts) and over-attributes by
  an order of magnitude. Prefer a same-day delta on a day with no other activity.
- **Runs from here on are TAGGED, so the three-filter reconciliation is a fallback, not
  the method** (issue #74, PR #75, `internal/model/costtag.go`). `-cost-tag` (default
  `go-cascade`) bills each run against a tagged **application inference profile** whose
  ARN goes out as `ConverseInput.ModelId`; the key is the account's already-ACTIVE
  `Project`, because a fresh cost-allocation key attributes nothing until activated and
  **activation does not backfill**. Pass a per-experiment value (`-cost-tag exp-31`) to
  separate one run from another. This does **not** retroactively attribute anything: the
  ~$197 to date and experiment 30's ~$1.20 stay in the untagged bucket, which is the
  motivation — this account's `Amazon Bedrock Service` line ran **$1126.23 in one day**,
  all untagged. Three things not to undo: the ARN is created only at the **provider
  boundary** (`Router.contaminated` enforces invariant #3 by string equality on model IDs
  and that comparison *fires* in the default config, so an ARN upstream silently stops
  flagging contamination with no test failing — pinned by
  `TestContaminationComparesLogicalModelIDs`); resolution **never fails a run** (falls
  back to the bare ID, warns once per model, caches the fallback); and
  `ConverseInput.RequestMetadata` is **not** the mechanism despite looking like it — it
  filters *invocation logs*, which are not even configured here, so it would have
  attributed nothing and returned no error. Still unverified: **one ~$0.01 probe per ARN
  family** to confirm a tagged profile ARN is accepted by `converse` for both the `us.*`
  Claude and bare-ID open-weight shapes; Cost Explorer's ~1-day lag means the attribution
  itself is only confirmable the following day.
- Bedrock models ACTIVE (us-west-2): maverick-17b, haiku-4-5, sonnet-4-5, sonnet-4-6,
  opus-4-5 (+ newer opus/sonnet/fable-5 profiles — keep configs pinned to the models
  each experiment used, for cross-run cost comparability). Re-run `go-cascade models`
  to confirm before any live run.
