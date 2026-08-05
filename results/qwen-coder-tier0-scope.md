# Scope: Qwen3 Coder at tier 0 (proposed, unrun)

Status: **scoped, not approved, not run.** Config lives at
`examples/bench/config.qwen-coder-211.json`.

## Why this arm, and why it is sharper than the previous cheap-tier arms

The cheap bottom tier is the **only cost lever in this study that has ever
worked**: experiment 11 put Llama 4 Maverick at tier 0 and the cascade beat
always-frontier 3.2–3.4×. Every other lever failed and failed the same way — it
bought accuracy with money. An Opus planner made the cascade 3.1× *pricier*
(experiment 15), a Haiku planner did not rescue it (16), and plan-once-reuse was
negative (18) because amortisation needs the cheap tier to *accept*.

Qwen3 Coder 30B A3B is the first candidate that removes the confound, because it
is **cheaper than the incumbent, not merely better**:

| | in $/MTok | out $/MTok | measured $/sample |
|---|---|---|---|
| Llama 4 Maverick 17B (current tier 0) | **0.24** | 0.97 | 0.000174 (profiled, n=409) |
| **Qwen3 Coder 30B A3B** | **0.15** | **0.60** | **0.000092** (4-problem probe) |

Rates are real `us-west-2` on-demand figures from the AWS Pricing API filtered to
`regionCode=us-west-2`. The per-sample figure is measured, not derived: a 4-problem
probe through the actual coder prompt shape averaged 177 in / 109 out tokens.
**1.9× cheaper per sample.** So a win cannot be reread as "we spent more."

> **Correction to an earlier revision of this table, which claimed Maverick at
> $0.12/MTok in.** That is Maverick's **batch** rate
> (`USW2-Llama4-Maverick-17B-input-tokens-batch`); the Converse path bills the
> on-demand row, $0.24. `config.go-specialist-211.json` has always configured 0.24,
> so no published cost figure is affected — but the comparison it implies is wrong in
> the *conservative* direction. Qwen is cheaper on **both** legs (0.15 vs 0.24 in,
> 0.60 vs 0.97 out), not only on output. When pulling a rate for a new tier, filter
> the `usagetype` string: `list-foundation-models` returns four rows per Meta model
> and the batch/on-demand pair differ by exactly 2×.

It is also the first *coder-specialist* tier 0; every prior cheap tier was a
general instruct model.

## Infrastructure: no code change needed

Verified live — `qwen.qwen3-coder-30b-a3b-v1:0` answers through plain
`bedrock-runtime converse`, the same path `internal/model/bedrock.go` already
uses. Same for `qwen.qwen3-coder-480b-a35b-v1:0`, `moonshotai.kimi-k2.5`,
`zai.glm-4.7`, `mistral.devstral-2-123b`. There is no separate endpoint to
integrate. Model IDs are configuration, so this arm is a config file.

Caveat for whoever looks for it: **`go-cascade models` will not list these.** It
calls `ListInferenceProfiles`, which returns only `us.*` inference profiles; the
open-weight IDs are bare (`qwen.*`, `zai.*`, `moonshotai.*`). Use
`aws bedrock list-foundation-models --region us-west-2`.

## What this arm can and cannot test

**It cannot move certifiable α at 2:1:1, and no model can.** Invariant #9 makes
the routing score a Wilson lower bound, so tier 0's ceiling is set by **fan-out**,
not accuracy: 0.2698 / 0.4249 / 0.6488 at n=1/2/5. Confirmed against the n=409
records — tier 0's maximum observed score is exactly 0.42499, the unanimous n=2
ceiling, while tier-0 *accuracy* is already 0.7702. Under τ=[1,1] tier 0 can never
accept regardless of which model sits there.

So read this arm as a test of **escalation rate and mean cost/query**, not of α.
The falsifiable prediction: a coder specialist cuts escalations and mean cost while
leaving certifiable α unchanged. If α *does* improve, my reading of the ceiling is
wrong, which is the more interesting outcome. Pair with a 5:2:1 fan-out if the
acceptance question is the actual target.

## Cost

**Corrected, and it is not the number the prior docs would suggest.** Recorded
`tiers[].cost_usd` omits the spec/oracle call entirely — every `r.spec` caller
passes a throwaway `&Result{}` — so any estimate built from records understates the
bill badly. Cost Explorer for 2026-07-24…08-03 shows **~$197** of real go-cascade
spend against ~$40 claimed across `results/*.md`, and **91% of it is the oracle
model** (`Claude4.6Sonnet`, $154.89).

Measured live at **$0.0408/problem** for the pinned spec (~2,700 output tokens: two
Go test partitions at sonnet-4-6's $15/MTok out). Tier 0 is 3.4% of recorded tier
cost, so swapping it changes ~$0.17 directly — **the win is fewer escalations, not
a cheaper tier 0.**

| run | n | estimate |
|---|---|---|
| Hand-written set, execution oracle only | 64 | **~$3.20** |
| MultiPL-E Go, execution oracle only | 409 | **~$20** |
| MultiPL-E Go, paired `-compare` (adds judge) | 409 | **~$25–30** |

The oracle term dominates every row, and it is identical across arms — which is
also why it is excluded from arm (e)'s matched budget but not from its bill.

**Recommendation:** run the **64-problem hand-written set first at ~$3.20.** It is
cheap, it is the set that still has concurrency coverage (MultiPL-E Go has zero
concurrency problems, so the `-race` rung never fires there), and it answers the
escalation-rate question directly. Only scale to 409 if the escalation rate moves.

## Repro (when approved)

```bash
AWS_PROFILE=aws ./bin/go-cascade calibrate -provider=bedrock \
  -config examples/bench/config.qwen-coder-211.json \
  -bench examples/bench/combined.jsonl \
  -refs examples/bench -pin-api \
  -alpha 0.05 -delta 0.10 -baselines -resume \
  -records results/qwen-coder-211.records.json \
  -o results/qwen-coder-211.cert.json
```

> **`-refs examples/bench`, not `examples/bench/refs`.** An earlier revision of this
> doc said the latter, and it silently resolves **28 of 64** references: `hard/` and
> `scale/` carry their own `refs/` subdirectories, and `loadReferences` walks for
> `*/<id>/solution.go` from whatever root it is given. Pointed at `examples/bench/refs`
> the run pins 28 APIs, leaves 36 problems with **no oracle-soundness gate at all**
> (`validateOracle` returns `OracleSound` when `r.refs[id]` is missing, by design), and
> is not comparable to the matched Maverick arm, which used `examples/bench`. The
> `loaded N/64` line on stderr is the check — it is printed for exactly this reason.
> `-provider=bedrock` is likewise required; without it the run is the mock and no
> number from it means anything.

Then compare escalation rate and mean cost/query against
`results/go-specialist-211-pinned-n64.execution.json`, which is the matched
Maverick arm at the same fan-out on the same 64 problems.

## Hazards

- **A new tier-0 model perturbs which problems the `-refs` gate excludes.**
  Maverick's confident errors coincided with spec-model test bugs, so those
  problems were dropped as oracle-unsound. A differently-flawed tier 0 moves `n`.
  Quote the denominator.
- **Do not put a reasoning model at tier 0 without pricing a real sample.** Kimi
  K2-Thinking and DeepSeek-R1 emit long outputs, and output length drives
  per-sample cost far harder than the per-token rate. At $3.00/MTok out a "cheap"
  reasoning tier can cost more than the Claude mid tier.
- `mistral.devstral-2-123b` answers Converse but has **no us-west-2 on-demand
  pricing rows**, so its cost cannot be pinned. Repo practice is verified rates
  only — it is not configurable yet.
