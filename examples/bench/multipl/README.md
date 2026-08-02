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

## Three gates

**488 of 528 problems ingest**, and the 40 exclusions are all upstream defects in
MultiPL-E's Go transpilation, not artifacts of this script. A problem whose canonical
suite cannot compile — or cannot be satisfied — has no usable oracle, so it must be
dropped and *counted*: shipping it would hand the estimator a problem it can never
label, which reads as a model failure rather than a benchmark defect. Every exclusion
is named in the output, and the first two gates delegate the judgement to the Go
toolchain rather than a hand-rolled check.

**2 do not parse** (`gofmt -e`): `mbpp_563_extract_values` and
`mbpp_725_extract_quotation` emit Go string literals containing unescaped double
quotes (`candidate("Cortex "A53" Based"…)`).

**37 do not type-check** (`go vet` against a `panic()` stub with the pinned
signature). `go vet` and not `go build`, because build does not compile `_test.go`
files — a signature/suite mismatch clears build and only fails later at test-binary
link, which is the same reason vet is a rung on the verifier ladder. Three shapes:

- **Heterogeneous arguments** (12). The prompt declares `[]interface{}` but the suite
  passes `[]int{…}` in one case and `[]string{…}` in the next — e.g.
  `mbpp_390_add_string`. Go has no implicit conversion to `[]interface{}`, so **no**
  signature satisfies both cases. Worth stating because it looks like a
  signature-extraction bug and is not: extracting the type from the tests instead
  cannot fix it, since the tests disagree with each other.
- **Internally invalid literals** (23). `[][]int{[]interface{}{3, 4}}` in
  `mbpp_400_extract_freq` — invalid regardless of the function's signature, since the
  outer and inner element types contradict each other.
- **Two one-offs.** `mbpp_105_count`'s suite contains a literal `UNKNOWN` type (the
  transpiler failed to infer one and emitted its placeholder), and
  `mbpp_67_bell_number` expects a 55-digit integer constant that overflows `int`.

**1 is unsatisfiable** (`contradiction`, a pure text check): `he_92_any_int`'s table
asserts both `candidate(3, 4, 7)` → `true` and `candidate(3.0, 4, 7)` → `false`. Through
`func AnyInt(x float64, y float64, z float64) bool` those are *the same call*, so no
function can satisfy the suite — Python's `isinstance(3.0, int)` distinction did not
survive transpilation, taking with it the only thing the problem tested.

This gate was added after stage 2 paid three generation attempts to discover the same
fact empirically, and it is the one exclusion class that a *semantically* aware check
finds and the toolchain cannot: the suite parses and type-checks perfectly. It compares
numeric literals by **value** rather than spelling (`3` and `3.0` must collide — that is
the whole defect), which is sound because the signature has already type-checked against
the suite. It is deliberately conservative: only exact-duplicate argument lists with
differing expectations, never a *suspected* inconsistency. Verified against all 489
suites that clear the other gates — it fires on exactly one.

`mbpp_802_count_rotation` shows why the conservatism matters. Its expectations follow no
rule (`[3,2,1]` → 1 and `[1,3,2]` → 2, though **neither list can be rotated into sorted
order at all**, so the question its statement asks is not defined on those inputs). It
passes this gate and should: its arguments are all distinct, so a lookup table satisfies
it. Unsatisfiable and merely-wrong are different defects and only the first is decidable
here; `mbpp_802` is caught by stage 2 instead, as a problem no reference validates.

These gates are what make stage 2's bill honest. Without them those 40 problems are paid
for twice — once in tokens spent generating a reference that can never pass, and again as
permanently-red directories in the tree.

**n = 488 still clears the §5.5 bar of n ≥ 300 comfortably** — as do the 467 that survive
stage 2 (see below). That bar is the only threshold that matters here; the exclusions cost
margin, not the experiment. Any write-up must report 488/528 and why, not the headline 528
— a reader comparing against published pass@k numbers on this benchmark needs to know 40
problems are absent and that the absences are not random with respect to difficulty (they
cluster in MBPP's tuple-heavy problems, which transpile worst to Go).

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
are named in the output. Running §5.5 at n=450 with sound references beats n=489 with
39 quiet lies; `loadReferences` tolerates a missing reference per problem, and such an
id simply carries no oracle-soundness gate.

## Stage 2 result: 467 of 489, and what the 22 failures actually are

Measured, Opus 4.5, `--attempts 3`: **467 validated (95.5%)**. All three consistency
checks pass — `manifest.json`, `problems.jsonl`, and `refs/` all list the same 489 ids;
the 22 ids without a `solution.go` are exactly the 22 the report marks unvalidated; and
the tree is `gofmt` clean.

**Only one of the 22 is a model failure.** That distinction is load-bearing, so all 22
were diagnosed rather than filed as "hard". Reproducing an expectation exactly from a
wrong formula is strong evidence about where the defect lives — a model error does not
reproduce the oracle's output on every case.

- **Unsatisfiable through the pinned signature** (1) — `he_92_any_int`. The suite
  asserts `candidate(3, 4, 7)` → `true` **and** `candidate(3.0, 4, 7)` → `false`, but
  the signature is `func AnyInt(x float64, y float64, z float64) bool`, so those are the
  *same arguments* with opposite expectations. Python's `isinstance(3.0, int) == False`
  has no equivalent through a `float64` parameter. **No implementation can pass.**
  This one turned out to be *mechanically decidable*, so it is now caught at ingestion
  by a third gate (`contradiction`) rather than discovered by paying for three
  generation attempts — see "Three gates" below. **The benchmark is therefore 488
  problems, not 489.**
- **Self-inconsistent oracle** (1) — `mbpp_802_count_rotation`. `[3,2,1]` → `1` (the
  minimum is at index 2) but `[1,3,2]` → `2` (the minimum is at index 0). No single rule
  satisfies all five cases.
- **The oracle encodes an upstream reference bug** (6) — `mbpp_461_upper_ctr` ("count
  the upper case characters"; `"PYthon"` → **1**, though the answer is 2 — iterating
  from index 1 reproduces every case), `mbpp_430_parabola_directrix` (expectations are
  exactly `c-((b*b)+1)*4*a`, not a directrix by any definition), `mbpp_260_newman_prime`
  (index off-by-one: expects `7` where `17` is the term the statement describes),
  `mbpp_264_dog_age` (expectations are exactly `10.5*2 + (age-2)*4`), `mbpp_83_get_Char`
  (`"abc"` sums to 294, 294 mod 26 = 8 → `'i'`, but the oracle wants `'f'`; no obvious
  variant reproduces it), `mbpp_87_merge_dictionaries_three` (the expected map omits a
  key present in the inputs). A candidate that is *correct* fails these; one that
  reproduces the bug passes.
- **Statement contradicts the oracle** (3) — `mbpp_638_wind_chill` says "rounded to the
  **next** integer" but every expectation matches `round`, not `ceil`;
  `mbpp_164_are_equivalent` says "sum of the divisors" but needs *proper* divisors
  (23/47 → both 1); `mbpp_777_find_sum` asks for a sum of non-repeated elements but
  expects the sum of *distinct* elements.
- **Under-specified ordering** (4) — `mbpp_579_find_dissimilar`, `mbpp_769_Diff`,
  `mbpp_788_new_tuple`, `mbpp_229_re_arrange_array`. The statement asks for a set-like
  result; the oracle demands Python's incidental iteration order.
- **Float last-bit disagreement** (1) — `mbpp_742_area_tetrahedron`. `math.Sqrt(3)*a*a`
  in Go differs from CPython's expectation in the final ULP (`…894` vs `…896`). The
  suite compares `fmt.Sprintf("%v")` output, so there is no tolerance to widen.
- **Genuinely hard, and satisfiable** (1) — `he_116_sort_array` needs Python's
  `bin(-4) == "-0b100"` semantics (popcount of the *magnitude*). Verified by hand: a
  `math/bits` + `sort.SliceStable` reference passes the full canonical suite. Opus
  missed it in 3 attempts. **This is the only one of the 22 that is a real model
  failure.**

The remaining 5 (`mbpp_408_k_smallest_pairs`, `mbpp_452_loss_amount`,
`mbpp_468_max_product`, `mbpp_617_min_Jumps`, `mbpp_749_sort_numeric_strings`) show the
same shapes — ordering or a disputed formula — but were not each run to ground, so they
are counted as unclassified rather than assigned a bucket.

**Consequence for any §5.5 number.** These 22 must be *excluded*, not counted as
cascade failures — the majority are problems where correctness and passing the oracle
are opposite. They carry no reference and therefore no oracle-soundness gate, which is
the mechanism that already keeps them out of calibration (invariant #4), but a
*pass-rate* reported over all 489 would silently charge the router for upstream defects.
**n = 467** is the number to quote, and it still clears the §5.5 bar of n ≥ 300.

This also sharpens the "model-written, human-test-validated" caveat above. The upstream
suite is not merely *incomplete* as an oracle on this benchmark — on a measurable
fraction of problems it is **wrong**, and it is wrong in the direction that rewards
reproducing a Python bug. Stage 2's validation gate silently filtered those out, which
is the right behaviour and worth knowing: the 467 references are, by construction,
solutions that agree with the upstream oracle *including* wherever it is mistaken.

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
