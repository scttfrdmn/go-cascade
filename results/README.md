# Live evaluation results

This directory holds the first evaluation of go-cascade against a live model
(AWS Bedrock, Claude Sonnet/Haiku/Opus tiers). Until this work the project had
never sent a query to a real model — every number came from the deterministic
mock. These runs replace that gap with measurements, and with an honest account
of what they do and do not establish.

**One-line summary:** across seven live experiments the executable oracle was
sound every time (β = 0, realized risk = empirical risk) and **certified a
strictly lower risk bound than the judge oracle on identical candidates (α=0.27
vs 0.32)** — the paper's primary §5.5 outcome, demonstrated live. The judge's
*only* confirmed dangerous failure (passing wrong code) was a **data race** it
could not see by reading; everywhere else it erred by over-rejecting correct
code, which is what costs it the certification race. The §3.1 danger (a judge
certifying below its true risk) is **real but narrow** — it lives in
reading-invisible defects — but the judge loses to the executable oracle on
certification mainly by over-rejection, not over-acceptance.

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

## The seven experiments

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
| 7 | [certification comparison](certification-comparison-2026-07-25.md) | **lowest certifiable α per oracle** (paired replay) | **execution α=0.27 < judge α=0.32** — the §5.5 primary outcome |

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
   candidates.

## What is established

1. **Execution is sound in practice.** In every run the execution arm's realized
   (ground-truth) risk equalled its empirical risk. β = 0 is not just a
   construction; it held live, repeatedly.
2. **The executable oracle certifies a lower risk bound than the judge.**
   α=0.27 vs 0.32 (δ=0.10, n=28, identical candidates) — the paper's primary
   comparative claim, demonstrated. The gap comes mostly from the judge's
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

- **No *useful-α* certification.** The comparison is done (execution 0.27 < judge
  0.32), but both α values are far above the ≤ 0.05–0.10 range anyone would
  deploy. That is the sample-size ceiling: at n = 28 with these error rates,
  δ = 0.10 cannot be met at low α. Reaching α = 0.05 needs n ≥ 45 *clean*
  records (paper eq. 7). Scaling n is the open frontier.
- **The scar-free-race blind spot is one data point and was not reproduced.**
  η_fa > 0 was observed exactly once (the pilot's model-authored race). The
  race-seeded test (experiment 6) caught all 20 seeded races — but only because
  sync-*deletion* leaves a visible scar; it does not produce the scar-free,
  self-consistent racy code that was actually false-accepted. A **scar-free race
  operator** (narrow a critical section, swap a goroutine capture) is needed to
  probe that class. **This is the recommended next experiment.**
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

Total live spend for all seven experiments plus diagnosis: roughly $23–27.
