# Pilot benchmark

A diverse pilot benchmark for the live evaluation the paper specifies in §5.5.
It exists because the original `examples/problems.jsonl` is 18 rephrasings of a
single task, which cannot exercise a real cascade — and because the mock
provider only knows two problem shapes, so a *diverse* benchmark is only
meaningful against a live model.

## Contents

- **`problems.jsonl`** — 28 problems, one `{"id", "problem"}` per line. 8 are
  concurrency problems (`conc_*`). Every problem is well-posed, solvable in a
  single file using only the standard library, and phrased so a spec model can
  derive an unambiguous API contract.
- **`refs/<id>/`** — a reference implementation and test suite for each problem:
  `go.mod`, `solution.go` (stdlib-only, exported doc-commented function), and
  `solution_test.go` with visible (`TestV*`) and adversarial hidden (`TestH*`)
  cases. Each is its own module, so it does not enter the main module's build.
- **`validate.sh`** — compiles and tests every reference by execution (`go test`,
  plus `go test -race` for the concurrency problems). Run it after any change.

## What the reference implementations are for

Two things, neither of which is "the answer the cascade must produce":

1. **Solvability proof.** They demonstrate, by execution, that every problem has
   a correct single-file stdlib Go solution. `validate.sh` is that proof; it must
   stay green.
2. **Ground-truth labels.** The oracle-gap validation contribution (CONTRIBUTING
   #2, paper §3.7) needs problems with reference implementations to measure the
   *actual* false-acceptance rate against the mutation-score estimate. These are
   those references.

The cascade does **not** use these files at solve time. Its spec model generates
its own API contract and test partitions from the problem string; the reference
here may choose a different function name or signature, and that is fine — each
arm is internally consistent against its own generated contract.

## Running it

```bash
# Validate the references (offline, no model, no cost):
bash examples/bench/validate.sh

# Live comparison of the two oracles (needs Bedrock; costs real money):
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  -bench examples/bench/problems.jsonl -alpha 0.10 -delta 0.10 -compare
```

At n=28 the achievable certified alpha is bounded by sample size, not model
quality: even at zero observed errors, certifying alpha=0.10 needs n>=22 and
alpha=0.05 needs n>=45 (see the README's "what calibration found"). This pilot
targets alpha=0.10; a larger benchmark is required for tighter risk.
