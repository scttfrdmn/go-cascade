# Hard benchmark tier

A second benchmark tier built to test the one thing the pilot (`../problems.jsonl`)
could not: whether an LLM judge's **false-acceptance** rate (η_fa) can be driven
above its false-rejection rate (β), reproducing the §3.1 *danger* — a judge that
certifies **below** its true risk because it cannot see the defects it accepts.

The pilot's problems were reader-friendly, so the judge's β dominated η_fa and
the danger did not appear. These 12 problems instead concentrate on **defect
classes that are invisible to a code reader but caught by execution**:

- **Integer overflow** — `hard_num_mean_overflow`, `hard_num_midpoint`,
  `hard_num_pow_mod`. The naive `sum/len`, `(low+high)/2`, and `base*base`
  implementations read as obviously correct and fail only at extreme values.
- **Aliasing / mutation** — `hard_slice_no_mutate_reverse`,
  `hard_slice_dedup_inplace`, `hard_slice_partition_stable`. A reader rarely
  traces backing-array mutation or notices that a two-pointer partition is not
  stable.
- **Unicode / whitespace** — `hard_str_utf8_reverse`, `hard_str_word_count`.
  Byte-level and ASCII-only implementations look fine on the page.
- **Concurrency** — `hard_conc_rate_limiter`, `hard_conc_once_init`,
  `hard_conc_ordered_fanout`. Check-then-act races and run-twice bugs are
  invisible without the race detector or a concurrent stress test.

Each problem ships a reference implementation whose hidden tests are
**demonstrated to break a plausible naive solution** (see the commit that added
them). `validate.sh` compiles and tests every reference — plain, and under
`go test -race -count=5` for the `hard_conc_*` problems. All 12 pass.

## Running

```bash
# Offline validation (no model, no cost):
bash examples/bench/hard/validate.sh

# Live paired comparison on the hard tier (needs Bedrock; costs real money):
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.example.json \
  -bench examples/bench/hard/problems.jsonl -alpha 0.10 -delta 0.10 -compare \
  -records hard.json
```

The hypothesis under test: on this tier the judge should false-accept more
(subtle wrong code reads as correct), so its realized risk should exceed its
empirical risk — the gap the execution oracle does not have. Whether it actually
does is an empirical question; record the outcome honestly either way.
