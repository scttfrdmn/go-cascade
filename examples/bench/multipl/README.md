# MultiPL-E Go benchmark ingestion

Turns [MultiPL-E](https://huggingface.co/datasets/nuprl/MultiPL-E)'s Go variants
into the layout go-cascade's `calibrate`, `estimator`, and `solve` commands consume.
This exists so the §5.5 validation experiment can run at **n ≥ 300 on a standard
benchmark** rather than on this project's 64 hand-written problems — the single gap
between "demonstrated at n ≤ 64" and "validated" (paper §5.5, §5.6).

Nothing here is run by `make test` or CI: it is a one-time data-preparation step, and
stage 2 costs money.

## Two stages

```bash
# Stage 1 — free, deterministic, offline. Needs pyarrow and a go toolchain.
for f in humaneval-go mbpp-go; do
  curl -sfL -o /tmp/$f.parquet \
    "https://huggingface.co/api/datasets/nuprl/MultiPL-E/parquet/$f/test/0.parquet"
done
python3 ingest.py --parquet /tmp/humaneval-go.parquet /tmp/mbpp-go.parquet \
                 --out /tmp/mple

# Stage 2 — costs money. Check the size of the bill first.
python3 stage2_references.py --bench /tmp/mple --dry-run
AWS_PROFILE=aws python3 stage2_references.py --bench /tmp/mple --resume
```

Stage 1 emits `problems.jsonl` (the query stream), `manifest.json` (pinned
signatures), and per problem `refs/<id>/{go.mod,solution_test.go}`. Stage 2 adds
`refs/<id>/solution.go` and a `references.json` report.

**526 of 528 problems ingest.** The two exclusions are `mbpp_563_extract_values` and
`mbpp_725_extract_quotation`, whose upstream suites contain Go string literals with
unescaped double quotes and so are not valid Go. They are detected with `gofmt -e`,
reported by name, and left without files — a problem whose canonical suite cannot
compile has no oracle, and shipping it would hand the estimator a problem it can
never label, which reads as a model failure rather than a benchmark defect.

## Why references have to be generated, and what that costs in rigour

MultiPL-E ships prompts and tests but **no solutions**, and go-cascade needs one per
problem for two load-bearing reasons:

- `calibrate -refs <dir>` runs each reference through the *generated* suite; a
  refuted reference flags the record `OracleUnsound` and excludes it from calibration
  (invariant #4). Without references, the spec model's test bugs silently inflate
  measured risk — observed live, not hypothetical.
- the §3.7 estimator uses each canonical suite as an independent oracle and pins the
  API extracted from the reference so candidates compile against it.

So stage 2 generates each reference with a frontier model and **accepts it only if it
passes MultiPL-E's own human-derived suite by execution** (`go vet` then the full
`go test`, no `-run` filter). The suite was not written by the model and is the same
artifact other work on this benchmark scores against.

The honest description is therefore **model-written, human-test-validated** — *not*
"human-authored", which is what the 64-problem benchmark's references are. Any
write-up using this benchmark must say so, because a defect the upstream suite does
not catch passes into the reference unnoticed. That bounds reference quality by the
upstream suite's strength — the same bound every published pass@k number on this
benchmark already carries, but it is a real difference from the hand-written set and
must not be blurred.

Problems whose reference fails after `--attempts` tries keep **no** `solution.go` and
are named in the output. Running §5.5 at n=480 with sound references beats n=526 with
46 quiet lies; `loadReferences` tolerates a missing reference per problem, and such an
id simply carries no oracle-soundness gate.

## Divergences from upstream MultiPL-E

These change the published artifact, so they are recorded here and belong in
`docs/divergence.md`-style accounting for any result derived from this benchmark.

### 1. Exported names: `has_close_elements` → `HasCloseElements`

MultiPL-E emits snake_case, which in Go is unexported, and `prompt.ExtractAPI` keeps
only *exported* declarations. Left alone, `-pin-api` extracts an empty API and the
oracle-soundness gate can never reach a verdict. The rename is applied to the prompt,
the docstring's `>>>` doctest lines, and the test suite.

The rename lives in the ingester rather than in `ExtractAPI` on purpose: teaching
`ExtractAPI` about unexported functions would change what `-pin-api` means for the
64-problem results every prior pinned experiment reported. A benchmark-shaped problem
gets a benchmark-shaped fix, not a change to an invariant-carrying package.

Only the *pinned identifier* is renamed. Three statements still contain snake_case
prose (`brazilian_factorial(n) = n! * …` in `he_139_special_factorial` is a
mathematical definition, not the function being requested); rewriting those would
alter the published problem text for no gain.

### 2. Package name: `package <fn>_test` → `package solution`

The verifier writes a candidate `solution.go` beside the suite in one directory.

### 3. Test function name: `TestHas_Close_Elements` → `TestCanonical`

This one is a **safety** change, not cosmetics. The upstream name begins with
`TestH`, and the ladder's acceptance stage filters on `^TestH` while its visible
stage filters on `^TestV`. A canonical suite handed to either would therefore run a
silent *fraction* of itself.

That is not hypothetical: it is exactly the bug that made experiment 19's first run
measure η_fa against 40% of its oracle (222 of 370 canonical tests skipped, and by
construction the adversarial ones). `TestCanonical` matches neither filter, so any
future mis-routing trips the zero-tests-ran guard in `ladder.Run`/`Accept` and fails
**loudly** instead of quietly weakening the oracle. The canonical suite is consumed
by `Ladder.RunAllTests`, which applies no filter, so nothing needs a V/H split here.

### Not diverged, deliberately

- **`>>>` doctest lines are kept** in the statement. They are part of the published
  prompt; dropping them would make this an easier benchmark than the one others
  report.
- **MBPP's "write a gothon function"** phrasing is an upstream translation artifact
  and is left verbatim. Editing published prompt text costs comparability, which is
  worth more than the oddity.

## Shape of the upstream data

Verified across all 528 rows rather than assumed, since the ingester's regexes depend
on it: exactly one function per prompt; exactly one `TestX` function per suite (all
cases in a `[]test` table driven by `t.Run` subtests); no helper functions; no
declared types; `fmt` and `testing` are the only imports any suite needs.

One consequence worth knowing for reading `canonical_tests` in the estimator's
records: `go test -json` reports each `t.Run` subtest separately, so a problem with a
7-case table counts as **8** (the parent `TestCanonical` plus seven subtests), not 1
and not 7. Verified rather than assumed, because that field is what makes oracle
strength auditable and an off-by-a-parent reading of it would misstate exactly the
quantity the field exists to expose.

## Repository hygiene

`ingest.py` runs `gofmt -w` over everything it emits. That is not cosmetic: `make
check` and CI both run `gofmt -l .` over the whole tree, and upstream's suites are
indented with a mix of two and three spaces, so they are never gofmt-clean as
shipped. Without this, one generated directory fails the build for every unrelated
change. If `gofmt` is not on `PATH` the script warns loudly rather than emitting an
unformatted tree silently.

Ingestion is deterministic and sorted, so a re-run produces a byte-identical
benchmark and diffs stay reviewable.
