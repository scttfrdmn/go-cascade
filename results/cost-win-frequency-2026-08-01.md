# Cost-win frequency at α=0.05 (5:1:1): 2 of 6 draws — and both wins ride the soundness gate, not fan-out — 2026-08-01

Experiment 13 reported a 5:1:1 draw that certified at α=0.05 **and** beat
always-frontier on cost. Experiment 14 (a theorem + two more draws) showed that was
a **single-draw result that does not replicate**, and flagged that draws b and c ran
*without* `-compare`, so `true_correct` (β=0 ground truth) was inferred, not measured.
This experiment closes both gaps: **three fresh 5:1:1 draws (d, e, f), all with
`-compare`**, aggregated with the three earlier draws (a, b, c) into a six-draw
frequency estimate.

**Headline: 2 of 6 draws certify with a cost win — and the mechanism behind those
two wins is not what experiment 13 assumed.** Both winning draws won because their
confident-wrong cheap-tier answers happened to be *oracle-unsound-excluded*, not
because fan-out or a lucky-but-sound generation kept the cheap tier clean. The cost
win is real but rests on a soundness-gate coincidence, which is even more fragile than
"a lucky draw."

## The six draws

All at α=0.05, δ=0.10, `-pin-api`, on `combined.jsonl` (n=64). Cost figures are
re-derived from the committed records with `-from-records -baselines` so all six use
the identical derivation (this makes draw a read 1.62× here vs the 2.13× reported live
in experiment 13 — same records, the live baseline used a slightly different cost
mix; the *sign and ranking* are what matter and are stable):

| draw | `-compare` | valid | τ0 thresholds | cascade $ | frontier $ | cost win? |
|------|-----------|-------|---------------|-----------|------------|-----------|
| a (#13) | no | true | [0.4, 1] | $0.00528 | $0.00854 | **WIN 1.62×** |
| b (#14) | no | true | [1, 1] | $0.01476 | $0.00840 | no (1.76× pricier) |
| c (#14) | no | **false** | [0.3, 0] halts | — | $0.00848 | no cert (top-tier miss) |
| d (this run) | **yes** | true | [1, 1] | $0.01488 | $0.00851 | no (1.75× pricier) |
| e (this run) | **yes** | true | [1, 1] | $0.01513 | $0.00875 | no (1.73× pricier) |
| f (this run) | **yes** | true | [0.4, 1] | $0.00664 | $0.00847 | **WIN 1.28×** |

**2 of 6** certify with a cost win; **3 of 6** certify but collapse to `[1,1]` (full
escalation → pricier than frontier); **1 of 6** fails to certify (top-tier miss). β=0
held on every `-compare` draw (execution realized risk = empirical, 0/0); the judge
arm over-rejected on all three new draws (empirical 0.073–0.077 vs realized 0.000).

## Why the wins win: the soundness gate, not fan-out

The certified threshold is `[1,1]` (no cheap-tier acceptance, no cost win) whenever the
**clean calibration set** contains a confident-wrong tier-0 answer — one reproduced
unanimously at the 5-sample unanimity ceiling (0.649), which no threshold below the
ceiling can reject without also rejecting correct unanimous answers. The
`analyze_draws.py` aggregate across all six draws:

| draw | clean n | tier-0 confident-wrong **in clean set** | τ0 | win? |
|------|---------|------------------------------------------|-----|------|
| a | 53 | 0 | 0.4 | WIN |
| b | 53 | 1 (`scale_is_palindrome`) | 1.0 | no |
| c | 53 | 0 | (top-tier miss) | no cert |
| d | 52 | 2 (`str_rle`, `scale_is_palindrome`) | 1.0 | no |
| e | 55 | 2 (`scale_is_palindrome`, `scale_fibonacci`) | 1.0 | no |
| f | 50 | 0 | 0.4 | WIN |

The rule is exact: **0 confident-wrong in the clean set → τ0 < 1 → cost win; ≥1 →
τ0 = 1 → no win.** (0 acceptance-risk events on all six — the cheap tier never
*accepts* a truly-wrong answer; the danger is purely that a confident-wrong answer
blocks a low threshold.)

Now the catch. Both winning draws had confident-wrong tier-0 answers *overall* — they
were just **excluded by the oracle-soundness gate** before calibration:

- **Draw a:** `seq_longest_run` and `scale_fibonacci` were confident-wrong at the
  ceiling, both `oracle_unsound=True` (the spec model's generated tests were wrong),
  excluded → 0 confident-wrong in the clean set → τ0=0.4.
- **Draw f:** `seq_longest_run`, `scale_fibonacci`, `scale_caesar` — same story, all
  three oracle-unsound-excluded (draw f had **14** unsound exclusions, the most of any
  draw) → 0 confident-wrong in the clean set → τ0=0.4.

So the cost win did not come from Maverick being reliably right, nor from fan-out
splitting a flaky cluster. It came from Maverick's *confident mistakes* happening to
land on the same problems the spec model wrote *unsound tests* for, so the soundness
gate removed them for an unrelated (and correct) reason. On draws b/d/e, Maverick's
confident mistakes (`scale_is_palindrome`, `scale_fibonacci`, `str_rle`) were on
problems with *sound* oracles, so they stayed in the clean set and collapsed τ0.

## What this establishes

1. **The α=0.05 cost win is the exception, not the rule: ~2 in 6 (33%), 95% Wilson
   interval roughly 10–70% at n=6 — wide, but clearly not "reliable."** This confirms
   and quantifies experiment 14's "does not replicate." The binding constraint is the
   cheap tier's **confident-error rate on sound-oracle problems**, exactly as the
   experiment-14 headroom theorem predicted (fan-out cannot separate a confident wrong
   answer from a correct one).
2. **Both observed wins are soundness-gate coincidences, not robustness.** This is a
   *stronger* fragility claim than #14's "lucky draw": the win requires Maverick's
   confident errors to coincide with the spec model's unsound-test problems. That
   coincidence is not a property you can engineer or rely on — it is noise in *two*
   independent error processes lining up favourably.
3. **`scale_is_palindrome` is the recurring sound-oracle confident error** (clean-set
   confident-wrong in b, d, e — 3 of 6 draws). It, not `scale_chunk` (which drew flaky
   in every draw), is the single problem most responsible for the [1,1] collapses.
4. **The resume fix survived two more external kills.** Draw f was SIGTERM'd at 24/64
   and again at 61/64; two `-resume` re-runs finished it with zero lost records (now
   five real kills survived across the study).

## Honest limitations

- **n=6 draws is a small frequency estimate.** "2 of 6" is the point estimate; the
  interval is wide. The robust claim is the *mechanism* (confident-wrong-in-clean-set ⇒
  no win, exact across all six) and the *direction* (wins are rare and gate-dependent),
  not the precise 33%.
- **Draws a, b, c lacked `-compare`** (their `true_correct` was inferred from the
  oracle, not measured). The three new draws (d, e, f) have β=0 ground truth; the
  mechanism holds identically on both the inferred and measured draws.
- **The oracle-unsound exclusion count varies a lot draw-to-draw (9–14)** because it
  depends on the spec model's per-draw test generation, not just the coder. This
  variance is itself part of why the cost win is unpredictable — the calibration set
  composition shifts between draws.
- **Cross-draw risk is not comparable** (fresh draws); the per-draw cert/cost is
  within-draw, and the confident-wrong-vs-τ0 rule is the like-for-like signal.

## Reproduce

```bash
# The three new draws (live, ~$4-6 each paired with -compare; -resume on SIGTERM):
AWS_PROFILE=aws go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.go-specialist-511.json \
  -bench examples/bench/combined.jsonl -refs examples/bench -pin-api \
  -alpha 0.05 -delta 0.10 -compare -baselines \
  -records results/go-specialist-511-draw-d.execution.json    # then -e, -f; add -resume if killed

# Aggregate all six draws (free):
python3 results/analyze_draws.py \
  '5:1:1-a=results/go-specialist-511-pinned-n64.execution.json' \
  '5:1:1-b=results/go-specialist-511-draw2.json' \
  '5:1:1-c=results/go-specialist-511-draw3.json' \
  '5:1:1-d=results/go-specialist-511-draw-d.execution.json' \
  '5:1:1-e=results/go-specialist-511-draw-e.execution.json' \
  '5:1:1-f=results/go-specialist-511-draw-f.execution.json'
```

Committed: `go-specialist-511-draw-{d,e,f}.{execution,judge}.json`. Live spend this
experiment: three paired n=64 draws with `-compare`, ~$12-15 total (draw f took three
launches after two external kills, but resume means no recomputation of recorded
problems).
