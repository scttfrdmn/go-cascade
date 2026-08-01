# The §3.7 estimator test: does mutation score predict *measured* η_fa? — design, 2026-08-01

This is the **design and tooling** for the §3.7 estimator test (paper §5.5(5)),
one of the two secondary experiments the live evaluation left unrun (§5.6). It is
written up **before** any live spend so the methodology — in particular the
non-circularity argument — is on record and reviewable. No live numbers here yet.

## The question, and the trap

§3.7 estimates the oracle gap η_fa = Pr[Y=0 | V=1] (a wrong program passing the
suite) by **mutation score** M = killed / valid. The paper is explicit that

> **M is a proxy for 1 − η_fa with unknown bias, not an unbiased estimator of it.**

The estimator test asks whether M actually tracks η_fa. The trap is **circularity**:
M is, by construction, `1 − η_fa` measured *on the mutation-operator defect
distribution* — a mutant that survives is exactly a wrong program the suite
accepts. If we then "measure η_fa" against those same mutants we reproduce M
tautologically and prove nothing. The whole point of §3.7's "unknown bias" is that
the model's real defect distribution (whole-algorithm errors, spec misreads) is
**not** the operator distribution. So the measured η_fa must come from an oracle
the mutation operators never touched.

## The independent oracle

Each benchmark problem ships, next to its `solution.go`, a **human-authored**
`solution_test.go` (the reference suite validated by `validate.sh`). This suite is
independent of the spec model's *generated* suite in two senses: a different author
(human vs. the spec LLM) and a different construction (written against the
reference, not sampled per run). It is the ground-truth oracle for Y.

Per (problem, tier) whose representative the **generated** oracle accepted (V=1):

- **M** = mutation score of that accepted candidate against the **generated**
  suite (the estimator whose bias we are testing).
- **Y** = correctness from the **canonical** reference suite (the independent
  label the operators never saw).
- **η_fa event** = `V=1 ∧ Y=0`: the generated oracle accepted a candidate the
  canonical suite refutes.

Correlating M against the rate of η_fa events across problems is then non-circular:
M is computed on one suite, η_fa is judged by a different one.

### Why this needs `-pin-api`

The canonical suite calls the reference's exact exported names. A candidate only
compiles against it if it implements that same API — which is exactly what
`-pin-api` forces (the spec model writes tests, and the coder writes code, against
the reference's pinned signatures). Without pinning, a name mismatch makes the
canonical suite fail to *compile*, which is **not** an incorrectness signal — the
tooling reports that row as `canonical_ran = false` (no label), never as a false
acceptance, mirroring how the oracle-soundness gate treats the same compile-vs-behaviour
distinction.

## What the tooling does (this PR, mock-tested only)

`go-cascade estimator -bench <probs> -refs <dir> -pin-api` (new subcommand):

1. Loads reference solutions (for API pinning) and their canonical test suites
   (the independent oracle) from `refs/<id>/`.
2. For each problem, profiles every tier on the cache-bypass path (as calibration
   does), and for each accepted representative records `EstimatorObs`:
   `generated_accept` (V), `canonical_correct`/`canonical_ran` (Y), `false_accept`
   (the η_fa event), and `mutation_score`/`mutation_valid`/`mutation_killed` (M).
3. Checkpoints observations after every problem and supports `-resume` (long live
   runs get SIGTERM'd externally — proven repeatedly on the calibrate path).
4. Prints the headline contingency: measured η_fa overall, and split at M ≥ 0.90
   vs M < 0.90. It deliberately reports the **raw contingency, not a correlation
   coefficient** — n is small and an over-precise statistic would misrepresent it.

It is a **measurement, not a certificate**: `EstimateOracleGap` never feeds a
threshold or the risk budget (invariants #4/#6). The second (canonical) oracle
exists only to audit the first; it is off the acceptance path entirely.

## What a run would (and would not) establish

- **Would:** on the existing 64-problem set, whether high-M candidates have
  systematically lower measured η_fa than low-M ones — the first *empirical* read
  on §3.7's "unknown bias," where the direction of the bias becomes visible.
- **Would not:** validate M as an unbiased estimator (n is small; the defect
  distribution is this benchmark's, not a general one), nor change any certificate
  — the certified bound already treats M as descriptive, not load-bearing.

## Cost and next step

A live run is one profiling pass over the benchmark with mutation analysis on
every accepted candidate — comparable to a single prior calibration draw (order
~$4–8 at n=64 with the Maverick/Claude tiers; more with mutation on the accurate
tiers). **Not yet run — scope the exact command and spend before launching.**

Reproduce the tooling offline (free):

```bash
go-cascade estimator --provider=mock -bench examples/bench/problems.jsonl \
  -refs examples/bench -o estimator.json
```

The mock cannot produce a meaningful η_fa (it returns one canned solution
regardless of problem, so most canonical suites do not compile against it — every
such row is correctly `canonical_ran=false`, not a false accept). It exists to
exercise the code path, per the standing rule: **never cite a mock number as a
measurement of model behaviour.**
