# Cost baseline: cascade vs single-model policies — 2026-07-25

The project's central unrun experiment (README "known gaps"): the cascade is only
worth its complexity if it is **cheaper than always calling the frontier model at
no worse correctness**, and **more correct than always calling the cheapest**.
Every other result measured certifiable *risk*; this measures the actual value
proposition — cost and ground-truth correctness of three routing policies on the
same n=64 records, computed offline (free).

## Result (n=64, ground-truth correctness, execution oracle)

| policy          | risk (1−accuracy) | mean cost / query | verdict |
|-----------------|-------------------|-------------------|---------|
| always-cheapest | 0.1094            | $0.00784          | riskiest |
| cascade         | 0.0938            | $0.00928          | **most expensive** |
| always-frontier | 0.0938            | $0.00789          | **best here** |

**On this benchmark the cascade does not win.** Always-frontier matches the
cascade's risk (0.094) at lower cost ($0.0079 vs $0.0093), and beats
always-cheapest on both. The cascade is the most expensive of the three.

## Why — and it is a real, generalizable finding

The three tiers cost about the same per problem (~$0.008 each), even though Opus
is ~5× Haiku's per-token price. Two effects erase the price gradient:

1. **The sampling policy inverts it.** The cheap tier draws **5 candidates**, the
   mid tier 2, the frontier tier **1** (default config). So 5× Haiku calls ≈ 1×
   Opus call in token cost — the cheap tier's per-token advantage is spent on
   drawing 5× as many samples.
2. **Verification cost is tier-independent.** Compiling, testing, and race-
   checking a candidate costs the same whether Haiku or Opus wrote it, and on
   this stdlib-Go benchmark that compute is a large share of per-query cost.

Confirmed directly from the records — **per sample** the price gradient is
intact, and it is the fan-out that flattens it per problem:

| tier | samples | cost/problem | cost/sample |
|------|---------|--------------|-------------|
| small (Haiku)  | 5 | $0.00784 | **$0.00157** |
| mid (Sonnet)   | 2 | $0.00920 | $0.00460 |
| large (Opus)   | 1 | $0.00789 | **$0.00789** |

Per sample, Opus is ~5× Haiku — exactly the token-price ratio. The 5:2:1 fan-out
is what erases it at the problem level.

The cascade then pays the cheap-tier sampling cost **and** escalates on ~40% of
problems, so it accumulates more than always-frontier while landing at the same
risk. Always-frontier wins because, here, the frontier model is barely more
expensive per query yet solves more at the first try.

## What this means (honestly)

- **The cascade's cost advantage is not demonstrated on this benchmark — the
  opposite is.** This is the measurement the paper's own §5 flagged as missing,
  and run honestly it does not favour the design at n=64 on stdlib Go.
- **This is config- and workload-specific, not a refutation of the idea.** The
  cascade wins when the frontier model is *much* pricier per query than the cheap
  one — which requires (a) fewer cheap-tier samples (the 5:2:1 fan-out is what
  kills it here), and/or (b) a workload where verification is cheap relative to
  the model, and/or (c) a larger price gap between tiers than Haiku→Opus. The
  README's economics section already argues the design is Go-favourable *because*
  verification is cheap; this shows the sampling fan-out can still overwhelm that.
- **The lever is the fan-out.** Reduce cheap-tier samples (e.g. 1:1:1) and the
  cheap tier gets genuinely cheap again; the cascade's cost should drop below
  always-frontier while escalation preserves risk. That is the obvious next
  experiment, and it is a **config change, not code** (`Samples` per tier).

## Ties to the two open questions this session raised

- **A cheap domain-specialist bottom tier** (e.g. a Go-only model) would help here
  precisely by making tier 0 both cheaper and more accurate, so fewer problems
  escalate and the cheap samples cost less — directly attacking both terms that
  make the cascade lose today.
- **Reducing the sample fan-out** is the cheapest immediate test of whether the
  cascade can beat always-frontier at all on this benchmark.

## Reproduce (offline, free)

```bash
go-cascade calibrate -from-records results/scaled.execution.json \
  -alpha 0.19 -delta 0.10 -baselines -o /tmp/cert.json
```

`Baselines` (internal/calibrate) computes all three policies from the per-tier
cost and ground-truth correctness already in the records; no model calls.
