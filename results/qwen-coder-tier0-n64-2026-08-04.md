# A coder-specialist tier 0 is *not* a cost lever: Qwen3-Coder 30B A3B at n=64 — 2026-08-04

Issue #61. Experiment 26.

The cheap bottom tier is the only cost lever in this study that has ever worked
(experiment 11, 3.2–3.4×). Every other lever bought accuracy with money and lost. This
arm was the sharpest remaining test of that lever because Qwen3-Coder 30B A3B is
**cheaper than the incumbent, not merely different**, so a win could not be reread as
"we spent more" — and it is the first *coder specialist* at tier 0, where every prior
cheap tier was a general instruct model.

**Headline: the prediction was wrong in both directions, and the negative result is the
useful one. A coder specialist did not raise cheap-tier accuracy — it *lowered* it,
0.9149 → 0.8298 paired (McNemar 5-vs-1, p=0.22, so directionally clear but not
significant at n=47). Escalations rose (3 → 7 at τ=[0.1,·]). And the arm certifies α=0.10
where the incumbent certifies α=0.05 — not because tier 0 is worse, but because the
*frontier* tier missed a problem. "Specialist" is not a synonym for "better at the thing
you are measuring."**

> **n = 47 paired. This is one draw at a sample size experiment 17 already showed is too
> small to settle the cost question: across six draws at this n, 2 of 6 certified with a
> cost win, 3 of 6 certified-but-pricier, 1 of 6 failed to certify. Read the direction
> and the mechanism, not the point estimates.**

## Setup

```bash
AWS_PROFILE=aws ./bin/go-cascade calibrate -provider=bedrock \
  -config examples/bench/config.qwen-coder-211.json \
  -bench examples/bench/combined.jsonl \
  -refs examples/bench -pin-api \
  -alpha 0.05 -delta 0.10 -baselines -resume \
  -records results/qwen-coder-211.records.json \
  -o results/qwen-coder-211.cert.json

# every comparative figure below, free, from the committed records
python3 results/compare_tier0.py -pair-out /tmp/qwenpair \
  results/go-specialist-211-pinned-n64.execution.json \
  results/qwen-coder-211.records.execution.json
go-cascade calibrate -from-records /tmp/qwenpair.{A,B}.json -alpha X -delta 0.10 -baselines
```

- 64/64, 54 min, no kill, launched detached (the harness reaper has SIGKILLed six long
  runs in this study). One problem, `conc_safe_counter`, alone took ~5 min blocked on
  Bedrock at 0% CPU — worth noting only because it is the problem whose *generated test*
  hung in experiment 25, and this time the delay was upstream latency rather than the
  oracle. The two look nothing alike once you check CPU time.
- Config identical to `config.go-specialist-211.json` except tier 0's model, so the
  comparison is like-for-like: 2:1:1 fan-out, `test_model=sonnet-4-6` (differs from every
  tier, invariant #3), same 64 problems, same `-pin-api` gate.
- **Bill: $0.90 recorded tier cost + ~$2.61 unrecorded spec/oracle ≈ $3.5**, against
  ~$3.6 scoped and $3.20 in the issue. The spec term is **74% of it** and appears in no
  `Record` — see `results/README.md` §"A correction to every cost figure on this page".
  Qwen's own tier-0 line is **$0.027 of the $3.5, i.e. 0.8%**, which is the whole reason
  a cheaper tier 0 cannot pay for itself.

## Two defects in the scope doc, found before spending

Both would have produced a wrong number, and both are fixed in
[`qwen-coder-tier0-scope.md`](qwen-coder-tier0-scope.md).

1. **`-refs examples/bench/refs` resolves 28 of 64 references, not 64.** `hard/` and
   `scale/` carry their own `refs/` subdirectories and `loadReferences` walks for
   `*/<id>/solution.go` from whatever root it is handed. Pointed at the narrow root the
   run pins 28 APIs and leaves **36 problems with no oracle-soundness gate at all** —
   `validateOracle` returns `OracleSound` when `r.refs[id]` is missing, which is correct
   behaviour and is exactly what makes the mistake silent — and would not be comparable
   to the matched Maverick arm. The `loaded 64/64` stderr line is the check.
2. **Maverick's quoted "$0.12/MTok in" is its *batch* rate**
   (`USW2-Llama4-Maverick-17B-input-tokens-batch`); Converse bills the on-demand row at
   $0.24. `config.go-specialist-211.json` has always configured 0.24, so no published
   figure moves — but the error ran in the *conservative* direction. Qwen is cheaper on
   **both** legs (0.15 vs 0.24 in, 0.60 vs 0.97 out), not only on output.
   `list-foundation-models` returns four rows per Meta model and the batch/on-demand pair
   differ by exactly 2×; filter on `usagetype`.

Qwen's rates verified against the Pricing API (`regionCode=us-west-2`): $0.00015/1K in,
$0.0006/1K out standard on-demand, matching the config exactly.

## Result 1 — the specialist is *less* accurate at tier 0

Paired over the 47 problems usable in **both** arms:

| tier 0 | accuracy |
|---|---|
| Llama 4 Maverick 17B (incumbent) | **43/47 = 0.9149** |
| Qwen3-Coder 30B A3B | **39/47 = 0.8298** |

McNemar on the discordant pairs: **5 Maverick-only vs 1 Qwen-only, exact two-sided
p = 0.2188**. So the direction is clear and the magnitude is not established at this n —
which is the honest reading, and is still enough to refute the arm's premise, because the
prediction was that a specialist would *raise* cheap-tier accuracy.

The four extra misses (`scale_caesar`, `scale_fizzbuzz`, `slice_most_frequent`, `str_rle`)
are not exotic; they are the middle of the benchmark. A 30B coder model is not uniformly
stronger than a 17B general instruct model on short stdlib-only Go.

## Result 2 — escalations rose, so the cost win moved the wrong way

Tier 0 is 0.8% of this run's bill, so the arm could only pay for itself through **fewer
escalations**. It bought more:

| τ | arm | accept@tier0 | escalated | risk | $/query |
|---|---|---|---|---|---|
| [0.1, 1.0] | Maverick | 44 | **3** | 0.0213 | **$0.00139** |
| [0.1, 1.0] | Qwen | 40 | **7** | 0.0426 | $0.00286 |
| [0.4, 1.0] | Maverick | 39 | 8 | 0.0213 | $0.00320 |
| [0.4, 1.0] | Qwen | 39 | 8 | 0.0426 | $0.00309 |

At the threshold where the cascade is cheapest, Qwen is **2.1× more expensive at 2× the
risk**. Where escalation counts tie, cost ties. There is no threshold vector on this draw
where the specialist is both cheaper and no riskier.

The per-sample claim does hold, and is worth separating from the verdict, because it is
the one part of the arm's premise that survived:

| | $/sample (clean, median) | $/sample (mean, incl. repair) |
|---|---|---|
| Maverick | $0.000244 | $0.000251 |
| Qwen3-Coder | **$0.000172** | $0.000212 |

**1.42× cheaper per clean sample, 1.30× cheaper per sample overall.** The scope doc's
4-problem probe said $0.000092, which is 1.9× optimistic against the median — the probe
priced one clean generation and the configured `repair_depth: 2` means a failing candidate
costs additional turns. Tier-0 cost per problem ranges $0.000182–$0.002023, a 5.9× spread
around the median, entirely repair. **Price a tier at its median *with repair enabled*, not
from a clean probe.**

## Result 3 — the certificate moved, and *not* because of tier 0

This is the part that would be easy to report wrongly.

| α | Maverick | Qwen |
|---|---|---|
| 0.05 | **valid**, τ=[1,1], $0.01277 | **invalid** |
| 0.08 | **valid**, τ=[1,1], $0.01277 | **invalid** |
| 0.10 | valid, τ=[0.1,1], $0.00139 | valid, τ=[1,1], $0.01277 |
| 0.15 | valid, τ=[0.1,1], $0.00139 | valid, τ=[0.1,1], $0.00286 |

Read off the table, "the specialist arm cannot certify α=0.05" invites the conclusion that
its cheap tier is too weak. **That is not the cause.** Per-tier misses on the paired set:

| tier | Maverick misses | Qwen misses |
|---|---|---|
| small | 4 | 8 |
| mid | 1 (`hard_num_mean_overflow`) | **0** |
| large | **0** | **1 (`hard_num_mean_overflow`)** |

Qwen's mid tier is perfect and its **frontier tier misses one problem**. Since the final
tier has no threshold — invariant #6, by construction — a frontier miss is a risk no
threshold vector can route around, so it floors empirical risk at 1/47 = 0.0213 and α=0.05
becomes unreachable *whatever tier 0 does*. Both arms share the mid and large models and
the same config; the miss is a fresh sample from the same frontier model, i.e. draw noise,
and it is the same problem the Maverick arm's *mid* tier missed. `hard_num_mean_overflow`
is a hard problem for Claude, in both arms, at whichever tier happened to draw it.

So the certificate difference is attributable to a top-tier draw, not to the intervention
under test. Experiment 17 saw exactly this: of six draws at this n, one failed to certify
because of a top-tier miss. **A tier-0 intervention evaluated at n≈50 can be swamped by a
single frontier sample.** That is a statement about the experimental design, not about
either model.

## Result 4 — the confident-wrong class, and why a cost win here would have been unreadable

Experiment 17 established that the deployable-α cost win happens **iff** the clean
calibration set contains zero confident-wrong tier-0 answers — a unanimous cluster (score
at the fan-out ceiling, 0.4249 at n=2) on a program execution says is wrong. Experiment
14's headroom theorem is why: fan-out provably buys discrimination against *flaky*
cheap-tier errors and **none** against confident ones, so no threshold separates them.

| arm | confident-wrong in the paired clean set | confident-wrong among the oracle-unsound exclusions |
|---|---|---|
| Maverick | 1 (`scale_chunk`) | 3 |
| Qwen | 1 (`scale_caesar`) | 5 |

Both arms carry exactly one, so neither can reach the zero-confident-wrong condition on
this draw and neither gets the clean win. Note the second column: the class is **more
common among the excluded records than the included ones in both arms**, which is
experiment 17's coincidence — the cheap tier's confident mistakes keep landing on the
problems whose generated oracle is independently unsound. The exclusion set is not the
model's doing, so a cost win that depended on it would not be evidence of cheap-tier
robustness. `results/compare_tier0.py` prints this class on both sides of the gate for
exactly that reason.

Qwen's four extra misses split 1 confident / 3 unanimous-wrong-with-score-0 — i.e. mostly
cases where *both* samples failed the visible ladder, which the score already reports as
0 and a threshold handles correctly. Its one confident-wrong answer (`scale_caesar`) is
the one that costs.

## Result 5 — the denominators, and the trap this run walked into first

The two arms are separate live runs, so they share **nothing** — not the candidates, not
the generated tests, and **not the exclusion set**, because oracle soundness is a property
of the generated suite and the suite is regenerated per run.

| | records | usable | oracle-unsound |
|---|---|---|---|
| Maverick | 64 | 52 | 12 |
| Qwen | 64 | **56** | **8** |
| **paired** | | **47** | |

5 problems are usable only in the Maverick arm, 9 only in the Qwen arm. So the per-arm
figures — tier-0 accuracy **0.8846** over 52 against **0.8393** over 56 — are each correct
and **not comparable**: they describe different problem sets, and their 0.045 gap is half
the paired gap of 0.085. Reading one against the other is the same defect as the arm-(e)
summary bug, where two correct rates over different denominators read as a 0.19 gap where
the paired figure was 0.11. `compare_tier0.py` therefore computes everything over the
intersection and prints the per-arm rates only under a NOT-comparable header.

The escalation and cost figures are recomputed in the script because they are arithmetic
over recorded scores. The **certificates are not** — `-pair-out` writes each arm restricted
to the intersection and hands them to the shipped `calibrate -from-records`. A second,
untested copy of Hoeffding-Bentkus and the fixed-sequence grid ordering in an analysis
script is how invariant #7 gets quietly broken.

**Instability, third data point.** `conc_safe_counter` — the non-terminating
`TestHInt64Overflow` that was the entire headline of experiment 25, and the *one* id
flagged unsound in both prior draws — came out **sound here**. The spec model simply did
not write that test this draw. Meanwhile `num_isqrt` is newly unsound with a timeout, and
the two arms' unsound sets overlap on only 3 of 17 ids ever flagged. Experiment 19's
finding that **rejection-side rates are not stable at n=64** now reproduces across three
subcommands and two oracles. Treat every exclusion count on this page as evidence of a
large effect, never as a point estimate.

## What this establishes

1. **A coder-specialist cheap tier is not a cost lever on this benchmark.** Accuracy fell
   0.9149 → 0.8298 paired, escalations rose 3 → 7, and cost at the cheapest threshold rose
   2.1×. The premise — specialist ⇒ more cheap-tier acceptance — is refuted in direction,
   though not at significance (p=0.22).
2. **The per-sample saving is real and irrelevant.** 1.42× cheaper per clean sample, and
   tier 0 is 0.8% of the bill. Confirms the scope doc's own prediction that the win had to
   come from escalations; it did not.
3. **Repair, not the rate, dominates cheap-tier cost.** A 5.9× spread around the median,
   and a clean-sample probe was 1.9× optimistic. Price a tier with `repair_depth` set as
   configured.
4. **Experiment 11's 3.2–3.4× win remains the only cost lever that has ever worked, and it
   is still unexplained by model quality** — Maverick beats a purpose-built coder model at
   the same tier.
5. **A tier-0 intervention at n≈50 can be decided by a single frontier draw.** Qwen's
   α=0.05 failure is caused by one large-tier miss on `hard_num_mean_overflow`, not by its
   cheap tier. Any future cheap-tier arm should be read at α≥0.10, where the frontier miss
   is inside the budget, or run at n large enough that one top-tier sample cannot move the
   certificate.

## What this does not establish

- **Nothing about Qwen3-Coder as a model.** This is one draw, n=47 paired, on a
  single-file stdlib-only Go benchmark with a generated oracle, at a 2:1:1 fan-out whose
  Wilson ceiling (0.4249) caps what tier 0 can ever score regardless of accuracy
  (invariant #9). It is a measurement of *this configuration*.
- **Not that the 480B coder variant would behave the same.**
  `qwen.qwen3-coder-480b-a35b-v1:0` answers the same Converse path, but a larger model at
  tier 0 changes the cost premise entirely and would need its own rate check.
- **Not that certifiable α is insensitive to tier 0** — the scope doc predicted α would not
  move, and this run cannot test that prediction, because the α difference it produced has
  a frontier-tier cause. The prediction stands untested.

## Reproduce

```bash
# free, from the committed records
python3 results/compare_tier0.py -pair-out /tmp/qwenpair \
  results/go-specialist-211-pinned-n64.execution.json \
  results/qwen-coder-211.records.execution.json
go-cascade calibrate -from-records /tmp/qwenpair.A.json -alpha 0.05 -delta 0.10 -baselines -o /tmp/a.json
go-cascade calibrate -from-records /tmp/qwenpair.B.json -alpha 0.05 -delta 0.10 -baselines -o /tmp/b.json
```

Records: `results/qwen-coder-211.records.execution.json` (64), certificate
`results/qwen-coder-211.cert.json` (`valid=false` at α=0.05 — see Result 3 for why, and
note the cause is not the tier under test).
