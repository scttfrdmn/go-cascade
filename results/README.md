# Live evaluation results

This directory holds the first evaluation of go-cascade against a live model
(AWS Bedrock, Claude Sonnet/Haiku/Opus tiers). Until this work the project had
never sent a query to a real model — every number came from the deterministic
mock. These runs replace that gap with measurements, and with an honest account
of what they do and do not establish.

**One-line summary:** across twelve live experiments the executable oracle was
sound every time (β = 0, realized risk = empirical risk) and **certified a
strictly lower risk bound than the judge oracle on identical candidates, a gap
that widened with scale (α 0.27 vs 0.32 at n=28; **0.19 vs 0.30 at n=64**)** —
the paper's primary §5.5 outcome, demonstrated live. The judge's
*only* confirmed dangerous failure (passing wrong code) was a **data race** it
could not see by reading; everywhere else it erred by over-rejecting correct
code, which is what costs it the certification race. The §3.1 danger (a judge
certifying below its true risk) is **real but narrow** — it lives in
reading-invisible defects — but the judge loses to the executable oracle on
certification mainly by over-rejection, not over-acceptance. Two later
experiments moved the value story: a cheap non-Claude **bottom tier** at an
intermediate fan-out makes the **cascade beat always-frontier on cost by
3.2–3.4×** with the routing signal intact; and a **reference-validation gate**,
completed by **pinning the API** so it reaches every problem, showed the earlier
"~11% accuracy floor" was **~93% spec-model test noise** — the true model-error
rate is **~0.025 (1 in 40)**. That overturns the "floor is model accuracy"
reading and **relocates the deployable-α blocker back to sample size** (α=0.05
needs n≥45; the interrupted pinned run reached 40). The twelfth experiment ran
that pinned run to completion (n=64): **α=0.05 certifies** — the first
deployable-α certificate in the study, with genuine model errors **0/52** (the
floor was **100% test noise** on this draw) — **but the cost win inverts at that
α**: the certified thresholds `[1,1]` force the cascade to escalate every problem,
making it *pricier* than always-frontier. **Deployable α and the 3.2× cost win are
mutually exclusive at the 2:1:1 fan-out** — the cheap tier's 2-sample Wilson bound
cannot clear a threshold strict enough to certify α=0.05.

## The central question

The paper argues (§3.1) that an *executable* oracle can certify a lower risk
bound than an *LLM-judge* oracle, because execution is sound (a failed test
means the program is wrong, full stop) whereas a judge has an unknown
false-acceptance rate η_fa that inflates true risk above what it certifies. The
comparison was argued, never demonstrated. These runs demonstrate the pieces.

## Setup common to all runs

- Tiers: `claude-haiku-4-5` (small), `claude-sonnet-4-5` (mid), `claude-opus-4-5`
  (large). Oracle / judge model: `claude-sonnet-4-6` — distinct from every tier,
  or the contamination guard (invariant #3) would exclude every record.
- Benchmarks: a 28-problem pilot (`examples/bench/`) and a 12-problem hard tier
  (`examples/bench/hard/`), all single-file stdlib Go, references validated by
  execution.
- α = 0.10, δ = 0.10. At n ≤ 28 nothing certifies at α = 0.10 (the sample-size
  floor needs n ≥ 22 even at zero errors), so the *risk numbers* are point
  estimates; the *oracle-divergence* counts are the signal.

## The eleven experiments

Each row links to its full write-up. "η_fa" = judge passed a program the tests
refute (the dangerous direction). "β" = judge failed a program the tests accept.

| # | run | what it isolated | headline |
|---|-----|------------------|----------|
| 1 | [pilot](pilot-2026-07-25.md) | first live run, independent sampling | exec sound; comparison **confounded** by sampling |
| 2 | [paired pilot](pilot2-paired-2026-07-25.md) | oracle (shared candidate stream) | exec β=0 live; judge **η_fa=3 (a data race)**, β=8 |
| 3 | [hard tier](hard-paired-2026-07-25.md) | reader-invisible defects | judge **over-rejected** subtle code: η_fa=0, β=6 |
| 4 | [strictness sweep](judge-sweep-2026-07-25.md) | judge prompt (re-sampled) | η_fa seen but **not attributable** (confound) |
| 5a | [controlled sweep](sweep-controlled-2026-07-25.md) | strictness (shared stream) | knob isolated; η_fa **untestable** (all candidates correct) |
| 5b | [seeded test](seeded-2026-07-25.md) | strictness on **known-wrong** logic defects | judge caught **all 17**; η_fa=0 at every level |
| 6 | [race-seeded test](race-seeded-2026-07-25.md) | strictness on **seeded race** defects | judge caught **all 20** — but the operator leaves a scar (see caveat) |
| 7 | [certification comparison](certification-comparison-2026-07-25.md) | **lowest certifiable α per oracle** (paired replay, n=28) | **execution α=0.27 < judge α=0.32** — the §5.5 primary outcome |
| 8 | [scaled certification](scaled-certification-2026-07-25.md) | same, at **n=64** | **execution α=0.19 < judge α=0.30** — gap widens; floor is model accuracy, not n |
| 9 | [cost baseline](cost-baseline-2026-07-25.md) | **cascade vs single-model policies** (cost + truth) | at default 5:2:1, **always-frontier wins** — cascade is priciest; the fan-out erases the cheap tier's edge |
| 10 | [fan-out test](fanout-2026-07-25.md) | same, at **1:1:1** fan-out | **cascade wins on cost — 2.75× cheaper than frontier** — confirming the fan-out was the culprit (risk noisy) |
| 11 | [Go-specialist tier + oracle noise](go-specialist-2026-07-25.md) | cheap **non-Claude bottom tier** (Llama 4 Maverick) at **2:1:1 / 3:2:1**, then **API pinning** to close the oracle gate | **cascade beats frontier on cost 3.2–3.4× with sample count intact**; and pinning the API so the reference gate reaches every problem shows the "~11% floor" was **~93% spec-model test noise** — true model-error rate **~0.025 (1 in 40)**, relocating the deployable-α blocker back to sample size |
| 12 | [completed n=64 pinned run](pinned-n64-complete-2026-07-30.md) | the **full-n pinned run** experiment 11 left interrupted (deployable-α + cost, together) | **α=0.05 certifies** (first deployable-α cert; genuine model errors **0/52**, floor was **100% test noise**) — **but the cost win inverts there**: at α=0.05 the certified thresholds `[1,1]` force full escalation, so the cascade is **pricier** than frontier. Deployable α and the 3.2× cost win are **mutually exclusive at 2:1:1** |

### How each run answered the previous one's limitation

The value is in the chain, not any single number:

1. **Pilot** gave the first live risk (exec 0.14) but ran the two oracles on
   *independently sampled* candidates, so any execution-vs-judge difference was
   sampling noise, not the oracle. → build a paired comparison.
2. **Paired pilot** fixed that: both oracles ruled on the *same* candidates. Exec
   was sound (realized = empirical); the judge diverged in both directions —
   most importantly it **passed a data race** execution caught under `-race`.
   That is the §3.1 mechanism, observed. But the benchmark was reader-friendly, so
   β (8) outweighed η_fa (3). → build a harder benchmark to push η_fa up.
3. **Hard tier** (overflow, aliasing, Unicode, concurrency) *refuted the
   hypothesis*: the models actually solved these correctly, and the judge, unable
   to trace `math/big` and concurrent code, **over-rejected** (β=6, η_fa=0).
   Making code hard to read made the judge fail *safe*, not dangerous. → maybe the
   lever is the judge's own strictness.
4. **Strictness sweep** varied the judge's PASS/FAIL tie-break, but re-sampled per
   level, so it couldn't attribute η_fa to strictness (strict and permissive
   happened to judge equivalent candidates). → make it a true A/B.
5a. **Controlled sweep** judged one shared stream at every level. The knob was
   cleanly isolated (one correct program flipped FAIL→PASS as the judge loosened),
   but that run's candidates were *all correct*, so η_fa was forced to 0 —
   benchmark luck, not evidence. → seed wrong candidates deliberately.
5b. **Seeded test** put 17 *provably-wrong* candidates (killed mutants) in front of
   the judge at every strictness. The judge **failed all 17, at every level,
   including permissive.** For single-edit logic defects the danger is not
   reachable by loosening the judge — it reads these bugs reliably.
6. **Race-seeded test** targeted the one blind spot: 20 seeded race defects. The
   judge caught all 20 — **but** the sync-deletion operator leaves a visible scar
   (a WaitGroup with no Wait, a Lock with no Unlock), so the judge catches the
   imbalance, not the race. The pilot's false-accepted race was scar-free, a class
   this operator cannot produce. So this brackets rather than closes the question.
7. **Certification comparison** replayed the paired-pilot records offline across α.
   Execution certifies down to **α=0.27**, the judge only to **α=0.32** — the
   §5.5 primary outcome, in the executable oracle's favour, on identical
   candidates. But both α were far above deployable, blamed on n=28. → scale n.
8. **Scaled certification** at n=64 tightened execution to **α=0.19** and widened
   the gap over the judge to **0.19 vs 0.30**. But α=0.05 stayed out of reach, and
   the run showed why: execution's empirical risk held at ~0.11, so the ceiling is
   **model accuracy, not sample size** — a lever the earlier runs couldn't isolate.
9. **Cost baseline** finally measured the value proposition itself: cascade vs
   always-cheapest vs always-frontier, on cost and truth. At the default 5:2:1
   fan-out, **always-frontier won** — same risk as the cascade at lower cost —
   because 5 cheap calls cost as much as 1 frontier call. → test a flatter fan-out.
10. **Fan-out test** re-profiled at 1:1:1. The cascade **flipped to winning: 2.75×
   cheaper than always-frontier** at no worse risk. The cost disadvantage was a
   tuning artifact — the fan-out is the dominant cost lever. (Risk noisy at n=64;
   1:1:1 also starves the routing signal, so the sweet spot is intermediate.)
11. **Go-specialist tier** put Llama 4 Maverick (a cheap non-Claude coder, real
   Bedrock price $0.24/$0.97) in tier 0 at **2:1:1 and 3:2:1** — the intermediate
   fan-outs 1:1:1 skipped. The cascade **beat always-frontier on cost by 3.2–3.4×
   with the sample count intact** (no starved clustering). Chasing why trivial
   problems "failed at every tier" then exposed a confound behind *every* prior
   risk number: the oracle runs the **spec model's generated tests**, which can
   assert wrong values or wrong function names — labelling correct code as wrong.
   A new **reference-validation gate** (`calibrate -refs`) excludes problems whose
   tests refute a known-correct reference; doing so **certified α=0.19** on both
   configs where the untriaged runs could not — but adjudicated only ~40% (a
   reference/spec API-name mismatch left the rest inconclusive). → pin the API.
   **API pinning** (`-pin-api`) then fed each reference's exact signatures to the
   spec model, collapsing inconclusive to ~0 and letting the gate reach a verdict
   on the whole benchmark. It exposed a third noise class (tests missing an import)
   and revealed the true model-error rate is **~0.025 (1 in 40)** — the "~11%
   floor" was **~93% spec-model test noise**. That **relocates the deployable-α
   blocker back to sample size** (α=0.05 needs n≥45; the interrupted pinned run
   left 40), overturning experiment 8's "the floor is model accuracy."

### Two candidate levers — one now run, one still open

- **A cheaper/more-accurate bottom tier** (e.g. a Go-specialist small model):
  **run** (experiment 11). It delivered the cost win but *not* a lower floor — the
  floor is a top-tier property under a sound oracle, and the apparent excess was
  spec-model test noise, not the bottom tier.
- **A generalist-instructs-specialist arm**: a strong general model rewrites the
  problem into a precise spec/plan that a narrow code model executes. Testable in
  this harness as a two-stage tier. Both are config/prompt-level experiments the
  cost baseline and cert comparison could score directly.

## What is established

1. **Execution is sound in practice.** In every run the execution arm's realized
   (ground-truth) risk equalled its empirical risk. β = 0 is not just a
   construction; it held live, repeatedly.
2. **The executable oracle certifies a lower risk bound than the judge, and the
   gap grows with scale.** α 0.27 vs 0.32 at n=28; **0.19 vs 0.30 at n=64**
   (δ=0.10, identical candidates) — the paper's primary comparative claim,
   demonstrated and strengthening. The gap comes mostly from the judge's
   over-rejection (β) inflating its empirical risk, not from η_fa.
3. **The judge's one confirmed dangerous (over-acceptance) blind spot is a
   scar-free concurrency race** — the sole live η_fa, exactly the defect class a
   reader cannot verify. The executable oracle caught it under `-race`.
4. **For logic defects and races-with-a-tell, a strong judge is reliable and
   strictness-robust.** 37/37 such seeded defects caught, permissive included.

Together these make the paper's claim **more precise**: the executable oracle
both certifies lower *and* covers the judge's blind spot (reading-invisible
defects — scar-free races, and by extension aliasing / spec misreads). The
certification advantage on this stream is driven more by the judge's
over-rejection than by its rare over-acceptance — a mechanism the paper
underweights, but one that still favours the executable oracle.

## What is NOT established (open, honest)

- **Deployable-α certification is REACHED (experiment 12) — but it costs the cost
  win.** Experiment 8's "α=0.05 is unreachable; the floor is model accuracy
  (~0.11)" was an artifact of oracle noise. With the `-refs -pin-api` gate removing
  spec-model test noise, the completed n=64 pinned run has genuine model errors
  **0/52** and **certifies α=0.05** (valid=true, 0 empirical risk) — the first
  deployable-α certificate in the study, which the judge arm cannot match on the
  same candidates (its lowest empirical risk is 0.077). The *new* open edge is that
  at α=0.05 the certified thresholds are `[1,1]`, forcing the cascade to escalate
  every problem, so it becomes **pricier than always-frontier**: deployable α and
  the 3.2× cost win are **mutually exclusive at 2:1:1**. The next lever is the
  cheap tier's routing signal at strict α — a 2-sample Wilson bound cannot clear a
  threshold high enough to certify α=0.05, so a higher cheap-tier fan-out is the
  untested path to getting both. (Caveat: 0/52 is one draw with a wide interval.)
- **The scar-free-race blind spot is one data point and was not reproduced.**
  η_fa > 0 was observed exactly once (the pilot's model-authored race). The
  race-seeded test (experiment 6) caught all 20 seeded races — but only because
  sync-*deletion* leaves a visible scar; it does not produce the scar-free,
  self-consistent racy code that was actually false-accepted. A **scar-free race
  operator** (narrow a critical section, swap a goroutine capture) is needed to
  probe that class. **This is the recommended next experiment.**
- **Cost baseline: RUN (experiments 9–11) — cascade loses at default config,
  wins when tuned, and wins with a cheap bottom tier at a signal-preserving
  fan-out.** At the default **5:2:1** fan-out, always-frontier beat the cascade
  (same risk, lower cost) — the cascade was priciest
  ([`cost-baseline`](cost-baseline-2026-07-25.md)). Flattening to **1:1:1** flipped
  it (2.75× cheaper, [`fan-out test`](fanout-2026-07-25.md)) but starved the
  routing signal. A **cheap non-Claude bottom tier (Llama 4 Maverick) at 2:1:1 /
  3:2:1** got the best of both: **3.2–3.4× cheaper than always-frontier with the
  sample count intact** ([`Go-specialist`](go-specialist-2026-07-25.md)). So the
  cost disadvantage was a **tuning artifact** — the sample fan-out is the dominant
  lever. **Caveat that persists:** the *risk* column is sampling-noisy at n=64
  (frontier's own risk moved 0.094→0.172 across fresh draws — never compare risk
  across runs). The robust claim is **"much cheaper (1.4–5.3× across runs) at no
  systematically worse risk"**, not "strictly lower risk."
- **The measured risk floor was overwhelmingly spec-model test noise, not model
  accuracy (experiment 11).** The calibration oracle runs the *generated* tests;
  when the spec model asserts a wrong value, a mismatched function name, or a
  missing import it labels correct code as wrong. The `calibrate -refs` gate
  validates the generated tests against an execution-checked reference, and
  `-pin-api` feeds each reference's exact signatures to the spec model so the gate
  reaches a verdict on the whole benchmark (inconclusive fell ~57%→~0). The result:
  the true model-error rate is **~0.025 (1 in 40)**, versus a raw measured ~0.15
  and experiment 8's cited ~0.11. **This overturns experiment 8's "the floor is
  model accuracy" conclusion** — the floor was oracle noise, and with it removed
  the certifiable α is once again gated by **sample size** (α=0.05 needs n≥45; the
  interrupted pinned run reached 40). **Remaining caveat:** the clean floor rests
  on n=40 from an externally-interrupted run; a completed n=64 pinned run is the
  open step (and needs whatever keeps killing long jobs resolved).
- **Judge β depends on the prompt.** The judge ran with "when in doubt, FAIL";
  its false-rejection counts would move under a different operating point. The
  strictness knob exists (`--judge-strictness`) but was only exercised on small n.
- **Small n throughout.** Every count here has a wide interval. Treat directions,
  not magnitudes, as the finding.

## The honesty line this work holds

Two hypotheses were *refuted* by running the experiment (the hard tier did not
raise η_fa; loosening the judge did not make it pass seeded logic bugs), and two
*confounds* were found and fixed rather than reported as results (independent
sampling in the pilot and the first sweep). The numbers here describe live Claude
models on small, specific benchmarks — not a general claim about LLM judges, and
not the paper's full §5.5 experiment (400+ problems, five arms), which remains
unrun. What changed is that the project now has real evidence with known edges,
where before it had only a mock.

## Reproducing

```bash
# Offline (no cost): validate the reference benchmarks
bash examples/bench/validate.sh
bash examples/bench/hard/validate.sh

# Live (needs Bedrock; costs real money). test_model must differ from every tier.
AWS_PROFILE=<profile> go-cascade calibrate --provider=bedrock \
  --config examples/bench/config.example.json \
  -bench examples/bench/problems.jsonl -alpha 0.10 -delta 0.10 -compare -records run.json

# Paired oracle comparison, strictness sweep, and seeded dangerous-mode test:
#   add -compare (paired), -judge-sweep (strictness A/B), or -judge-seed N (seeded).
```

Raw records (`*.execution.json`, `*.judge.json`, per-level and seeded JSON) are
committed alongside each write-up so any α can be re-evaluated offline.

Total live spend for all eleven experiments plus diagnosis: roughly $65–69.
