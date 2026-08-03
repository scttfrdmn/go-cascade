# Live evaluation results

This directory holds the first evaluation of go-cascade against a live model
(AWS Bedrock, Claude Sonnet/Haiku/Opus tiers). Until this work the project had
never sent a query to a real model — every number came from the deterministic
mock. These runs replace that gap with measurements, and with an honest account
of what they do and do not establish.

**§5.5(1) is now met.** Experiment 21 ran the paired comparison at **n=409 usable on a
standard benchmark** (MultiPL-E Go), the bar every earlier run fell short of. The primary
claim replicates and strengthens — **execution certifies α=0.084, the judge α=0.226**
(1.6× → **2.7×** the n=64 margin) — execution was sound on **1096/1096** observations,
and **η_fa is measurable for the first time (11/1096)**. Two earlier headlines do not
survive the scale-up: **α=0.05 does not certify** at n=409 (the n=64 "0/52 errors" floor
was a small-sample artifact; the real model-accuracy floor is 0.0538), and the
certifiable-α-vs-cost-win tension **reproduces** rather than dissolving. Read the rest of
this file with that correction in mind.

**One-line summary:** across twenty live experiments (plus three decided offline for $0)
the executable oracle was
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
making it *pricier* than always-frontier. At 2:1:1 the certified thresholds `[1,1]`
forced full escalation, making the cascade pricier than always-frontier. The
thirteenth experiment raised the tier-0 fan-out to **5 samples (5:1:1)** and, on that
draw, got **both at once** — α=0.05 with τ0=0.4 (cheap tier accepted) and 2.13×
cheaper than frontier at 0 risk. But the fourteenth experiment (theorem + two repeat
draws) shows that win **does not replicate**: fan-out provably buys headroom against
*flaky* cheap-tier errors and *none* against *confident* ones, and across three fresh
5:1:1 draws only one certified with τ0<1.0 — the others hit a cheap-tier
confident-wrong answer (no cost win) and a top-tier miss (no certificate at all). At
n≈53 a single error anywhere moves the certificate, so the deployable-α cost win is
**achievable on a good draw, not guaranteed**; the binding constraint is model
accuracy at small n.

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

## The experiments

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
| 12a | [tension diagnosis](tension-diagnosis-2026-07-30.md) (offline analysis of #12) | **why** α=0.05 forces `[1,1]` | the tension is **one flaky cheap-tier answer**: `scale_chunk` is oracle-wrong yet unanimous at the 2-sample Wilson ceiling (0.425), indistinguishable from 40 correct unanimous answers → τ0 collapses to 1.0. **0 tier-0 acceptance-risk events** — the cheap tier is never confidently-wrong-and-accepted. Localises the fix to a **higher tier-0 fan-out** (lifts the ceiling + splits flaky clusters); `config.go-specialist-511.json` added to test it |
| 13 | [5:1:1 fan-out](fanout-511-2026-07-30.md) | the diagnosis's predicted fix — **tier-0 = 5 samples** | **on this draw**, α=0.05 certifies AND the cost win holds: τ0=**0.4** (cheap tier accepted), cascade **2.13× cheaper** than frontier at 0 risk, 0/53 errors, unanimity ceiling 0.425→**0.649**. **Confound flagged** (`scale_chunk` drew correct) — and experiment 14 shows this **does not replicate** |
| 14 | [confound close](confound-close-2026-07-31.md) | theorem + **2 repeat draws** | **the α=0.05 cost win is real but fragile — it does not replicate.** Theorem (free): fan-out buys headroom vs **flaky** wrong (gap 0.40→0.88 as N=1→10) and **none** vs **confident** wrong (gap 0 ∀N). Empirically, **3 fresh 5:1:1 draws → 3 outcomes:** [0.4,1] win / [1,1] no-win (a cheap **confident-wrong** `scale_is_palindrome`, 5/5) / **valid=false** (a **top-tier miss**). At n≈53 one error anywhere moves the certificate — binding constraint is **model accuracy at small n** |
| 15 | [two-stage tier](two-stage-arm-2026-07-31.md) | the last untested **lever** — Opus **plans**, Maverick **codes** | **negative result: generalist-instructs-specialist is an accuracy lever, not a cost lever — and at an Opus planner, a cost disaster.** The plan nudges tier-0 accuracy 0.88→**0.92** (one problem, noise-band) and does **not** fix confident errors (`scale_chunk` stayed 5/5-wrong with the plan). But the Opus call on every tier-0 query makes tier-0 cost **35×** higher → cascade **3.1× pricier than always-frontier**. A cheap-bottom-tier (#11) beats instructing a specialist |
| 16 | [cheap-planner two-stage](two-stage-haiku-2026-07-31.md) | the #15 follow-up — **Haiku** plans (not Opus), Maverick codes | **a cheaper planner mitigates but does not reverse the penalty.** Same accuracy nudge (tier-0 true-correct **0.946** ≈ Opus's 0.92 > no-plan 0.88; `scale_chunk` still confident-wrong). Haiku shrinks tier-0 overhead **35×→8.7×** and α=0.05 now certifies (clean draw, no top-tier miss) — **but the cascade is still 2.13× pricier** than frontier at the certified `[1,1]`, and only **1.17× cheaper** where tier-0 acceptance fires (α=0.15) vs the non-planned #12's **5.24×**. Two planner points (Opus 3.1×, Haiku 2.13×) **confirm the direction**: accuracy lever, not cost lever |
| 17 | [cost-win frequency](cost-win-frequency-2026-08-01.md) | 3 more 5:1:1 draws **with `-compare`** (β=0), 6 draws total — turns #14's "does not replicate" into a **frequency** | **2 of 6 certify with a cost win; 3 of 6 certify-but-pricier (`[1,1]`); 1 of 6 no cert.** Exact rule: win **iff 0 confident-wrong tier-0 answers in the clean set**. And the catch — **both** wins had confident-wrong answers that were merely **oracle-unsound-excluded** (Maverick's confident mistakes coincided with the spec model's unsound-test problems), so the win rides a two-error-process coincidence, not robustness. `scale_is_palindrome` is the recurring *sound-oracle* confident error. Resume survived 2 more external kills (5 total) |
| 18 | [plan-once-reuse](plan-once-reuse-2026-08-01.md) | the last untested two-stage variant — **one** plan/query threaded into **every** tier (PR #35), not per-tier | **negative, and it explains why: at 2:1:1 the amortisation the design targets never happens.** Tier-0 accuracy 0.885 ≈ no-plan 0.88 (*below* both per-tier arms — one general plan loses the per-tier specificity that carried the nudge); cost collapses to per-tier's (~2× pricier at certified α=0.15, 1.19× at α≤0.10). Plan-once pays off only if the cheap tier *accepts* (avoiding escalation + plan re-charges), but the 2-sample Wilson ceiling (0.425) is below every certifiable threshold, so tier 0 always escalates and the one plan buys nothing. **Three planner points now close the question: no plan-placement variant reverses "accuracy lever, not cost lever."** Ran clean, no external kill |
| 19 | [§3.7 estimator test](estimator-test-n64-2026-08-01.md) | the paper's other unrun secondary experiment — is **mutation score M a usable proxy for η_fa** on the *model's* defect distribution? Non-circular: M vs the **generated** suite, correctness vs the **human-authored canonical** suite. **Run twice**: the first pass's canonical oracle ran only its `^TestV` half (40% of tests); figures below are the **full-oracle re-run** (PR #42) | **M is loose by an order of magnitude, and the oracle's real error is over-rejection.** Measured η_fa = **0/145** (95% upper bound **0.020**) while pooled 1−M = **0.1014** predicted **~12** false acceptances (P(0 events \| M tight) ≈ 1×10⁻⁶). So §3.7's "unknown bias" has a measured **direction — conservative**, the safe way for a risk proxy to err; M was never affected by the oracle bug (`Mutate` uses no `-run` filter), so the predicted side is unchanged. But the *discriminative* question is **unresolved**: M spans 0.50–1.00 yet both buckets (M≥0.90 n=96, M<0.90 n=42) have 0 events, so bounds overlap — the 12 lowest-M rows are all canonically **correct** (small mutant pools, timing-dependent mutants), i.e. low M was an artifact, not a signal. Flip side: **4 confirmed false rejections (2.6% of rows with a candidate) vs 0 false acceptances**, plus 40 rows where the ladder left no candidate — a **new** hazard class, "sound-but-stricter-than-canonical", that neither §3 nor the `-refs` gate models, and which costs money not risk. **Two further findings from the re-run:** the corrected oracle produced **3** canonical refutations (all `TestH*` — an off-by-one at `MaxUint64`, an int64 overflow, an input-mutation check) that the 40% oracle called **correct**, so *oracle strength must be recorded, not assumed*; and the two draws agree on only **159/192 rows**, so **rejection-side rates are not stable at n=64** — the asymmetry's direction replicated, its magnitude (11→4) did not. First live proof of the **atomic-checkpoint fix** (PR #39): killed twice, resumed with 0 loss (first pass); re-run finished 64/64 uninterrupted |
| 20 | [absorption ceiling](absorption-ceiling-2026-08-02.md) | the **last** unrun secondary experiment — §5.5(4) cache-warmth (the direct §2.9 test), scoped at ~$7. Asks first whether the benchmark *can* exhibit the effect | **not runnable here, decided offline for $0 — and the reason is a stronger result than the experiment would have been.** On MultiPL-E Go: retrieval candidacy **464/488 (95.1%)** vs absorption ceiling **2/488 (0.4%)**, three orders of magnitude apart. §2.9's effect is driven by *absorption* (arm zero re-executes — invariant #5), so a 0.4% cache shifts no distribution and the paid run would have reported a **benchmark artifact as a null**. Ceiling is **exact, not sampled**: a differing signature cannot compile, so `manifest.json` reduces 118,828 pairs to 10 ordered transfers, all executed; 4 pass = 2 bidirectional pairs, both **upstream MBPP duplicates**. The finding: at the high end similarity is **anti-correlated** with transferability — the top pairs are *antonyms* (`minimum`~`maximum` **0.949**, the highest of all 118,828; `first_Digit`~`last_Digit`; `even_position`~`odd_position`), and the top same-signature pair `he_56`~`he_61` `CorrectBracketing` (0.836) is **retrieved and refuted** (`<>` vs `()`). **Invariant #5 measured, not asserted.** §2.9 itself is untouched; running §5.5(4) needs **duplicate injection** (absorption as a dial), not a bigger corpus — **done in experiment 22** |
| 21 | [§5.5 at n=409](s55-multipl-n409-2026-08-03.md) | **the §5.5(1) bar itself** — n ≥ 300 on a *standard* benchmark (MultiPL-E Go: HumanEval-Go + MBPP-Go, 488 problems, 409 usable), all arms paired. The one gap between "demonstrated" and "validated" | **the primary claim replicates and the margin widens monotonically with n: execution certifies α=0.084, the judge only α=0.226** (δ=0.10, identical candidates) — the exec/judge ratio goes 1.19× (n=28) → 1.58× (n=64) → **2.69×**, and the absolute gap 0.050 → 0.110 → **0.142**. Execution sound again and now emphatically: **1096/1096** tier observations agree with ground truth, 0 over-accept, 0 over-reject. **η_fa is finally more than one data point — 11/1096 over-acceptances**, with a cheap-tier gradient (small 8, mid 2, large 1) exactly as §3.1 predicts; but *this run's* records keep no candidate source, so its *defect classes are unrecoverable* and the reading-invisible mechanism stays **argued, not confirmed** — fixed going forward by `TierObs.DisagreementSource` (issue #49), which retains the program on every oracle/truth disagreement, readable via `classify_disagreements.py`. **α=0.05 does NOT certify here** (floor 0.0538 is genuine model accuracy, not oracle noise) — the n=64 "α=0.05 certifies" was a 0/52 small-sample artifact, and this is the more trustworthy result. Cost tension **reproduces at 6× scale**: `[1,1]` (1.6× pricier than frontier) below α=0.11, `[0.1,1]` (**2.2× cheaper**) at or above it. Exclusions are load-bearing — the 79 dropped have **frontier risk 0.785**. **Coverage gap: 0 concurrency problems, so the `-race` rung never ran.** $8.15, 488/488, no kill |
| 22 | [§5.5(4) absorption dial](README.md#experiment-22--554-the-29-shift-measured-as-a-controlled-dial-0-offline) | §5.5(4) proper: make absorption a **controlled dial** over the n=409 records and measure how the certificate degrades, with and without shadow sampling. Offline, $0, 60 rows | **§2.9 tested rather than assumed.** Under a head-shaped filter the certificate goes optimistic monotonically — gap **+0.0147 → +0.1486** as ρ goes 0.2 → 0.8, and at ρ=0.6 it promises α=0.10 and delivers **0.134**, an actually *violated* bound. **Uniform absorption is a null** (acc 0.7702 → 0.7439 across all rates, noise in both directions), so #52's own framing — inject duplicates uniformly — would have measured nothing and reported it as evidence; it ships as the explicit control, and its envelope — measured over **10 seeds × 4 rates**, **max |gap| = 0.0389**, not the 0.0267 a single sweep shows — is the yardstick. By it the effect is established at **ρ ≥ 0.6, not ρ ≥ 0.4**; the selective rows are seed-exact (a sort), but the control is not, and the control sets the bar. Cleanest row holds n **fixed** at 327: uniform certifies `[1,1]`, both selective patterns refuse — same n, α, δ, grid, so only the shift can explain it. Shadow sampling drives the gap to **exactly 0** and converts a silent violation into a visible **refusal to certify**, which is the correction working, not failing. Limit stated not buried: α=0.10 at δ=0.10 needs **n ≥ 22 even at zero errors**, so 3 small-ε rows are flagged `underpowered` rather than reported |
| 23 | [arm (e) feasibility](README.md#experiment-23--552-arm-e-self-consistency-at-matched-cost-0-so-far-offline) | §5.5(2)'s last unimplemented arm — self-consistency at matched cost. Asks first, for $0, what the matched budget actually buys | **the arm is implemented and the free check rules it out at every tier but the cheapest.** Matched budget $0.0101/q under τ=[1,1]; at the profiled 2:1:1 fan-out that buys median **49** cheap-tier samples (0.0% below a 3-vote), **2** mid (79.2% below), **1** frontier (**99.5%** below). So a frontier arm (e) at matched cost is **always-frontier relabelled** and a mid-tier one is a coin flip — run as §5.5(2) literally specifies it (tier unnamed) it would have **reported a degenerate configuration as a null about self-consistency**, the same trap as experiment 22's uniform absorption. Only the cheap tier is well-posed, and there it is exactly §3.5's contrast: **49 votes on how the code is written vs 2 on what it does**, same money. Built as a fair foil, not a strawman — normalised-source vote (formatting/comments/import order do not split it), **raw plurality mass** not the Wilson bound (invariant #9 governs the *routing* score; arm (e) crosses no threshold), and it never consults the verifier to pick a winner. Both selectors scored on the **same candidates at the same cost**, so the selector is isolated; cluster abstentions reported separately, not scored wrong (invariant #4). `selfconsistency` **refuses `-sample` on a ruled-out tier.** Paid pass scoped **$4.12, unrun** |

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

### Two candidate levers — both now run

- **A cheaper/more-accurate bottom tier** (e.g. a Go-specialist small model):
  **run** (experiment 11). It delivered the cost win but *not* a lower floor — the
  floor is a top-tier property under a sound oracle, and the apparent excess was
  spec-model test noise, not the bottom tier.
- **A generalist-instructs-specialist arm**: **run, and now closed** (experiments
  15, 16, and 18). A strong planner rewrites the problem into a plan a cheap coder
  (Maverick) executes. It is an **accuracy lever, not a cost lever** — the plan
  nudges tier-0 accuracy up slightly (0.88→0.92 with a per-tier Opus planner,
  0.88→0.946 with a per-tier Haiku planner, both noise-band) and does not fix
  confident errors, while a planner call on every cheap-tier query makes the cascade
  pricier than always-frontier: **3.1× with Opus (#15), 2.13× with Haiku (#16)**.
  **Experiment 18 tested the last variant — plan-once-reuse-across-the-cascade (one
  plan per query threaded into every tier, the structural change in PR #35) — and it
  is also negative, for a newly-identified structural reason.** Plan-once did not even
  reproduce the accuracy nudge (tier-0 0.885 ≈ no-plan 0.88, *below* both per-tier
  arms — one general plan trades the per-tier specificity that carried the nudge), and
  it collapsed to the *same* cost as per-tier planning (~2× pricier at the certified
  α=0.15, 1.19× at α≤0.10). The reason is the amortisation the design was built to
  capture **never happens at 2:1:1**: it pays off only if the cheap tier *accepts*
  (avoiding escalation and plan re-charges), but the 2-sample Wilson ceiling (0.425)
  sits below every certifiable threshold, so tier 0 is always escalated through and
  the one plan buys nothing. **Three planner points on the same side close the
  question: no plan-placement variant reverses the direction.** The cheap-bottom-tier
  lever (11) remains the only thing that makes the cascade beat always-frontier on cost.

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

## A correction to the setup, found while building §5.5(4) (no experiment, no cost)

**The arm-zero *solutions* cache was structurally dead in every run above.** Not
cold, not underused — unreachable. `Router.finish` gates admission on
`res.Score >= cache_admit_score`, and `res.Score` is a Wilson **lower** bound on
cluster mass, not the raw mass (invariant #9). Its ceiling for a *unanimous* tier is
0.2698 at 1 sample, 0.4249 at 2, 0.6488 at 5 — reaching the shipped default of
**0.90 needs 25 samples**. All nine shipped configs run fan-outs of 1–5, so the
admission branch never executed and the solutions layer never held an entry.

Nothing failed, which is the point: an empty cache simply escalates, and that is
indistinguishable from a cold one. Found by structural audit, not by a test.

**What it does and does not invalidate.** No number above changes. Every records
file bypasses the cache by invariant #8 (`cmdCalibrate` forces `cache_dir=""`), and
all of them show `cache_hit=0` — verified across the 4 live records sets and the
estimator runs. The *spec* and *failure* layers are keyed exactly and not
score-gated, so the measured spec-cache saving ($0.0084 → $0.0028) is real. What was
dead was solution reuse specifically, which no published result rests on.

It mattered for what came next: experiment 20 concluded §5.5(4) needs duplicate
injection to make absorption a controlled dial, and the dial was wired to a branch
that could not fire. A paid run through that config would have reported "no §2.9
effect" from a setup incapable of exhibiting one — the same failure mode experiment
20 caught for a different reason, one layer down.

**Fixed** (`Config.DefaultAdmitScore`, `Config.Validate`): the default is now derived
as unanimity at the cascade's **narrowest** fan-out, and an explicitly unreachable
threshold is rejected outright rather than silently ignored. Keying it to the
*widest* tier is the tempting fix and is also wrong — acceptance usually lands on the
final tier, which is both narrowest and (by construction, invariant #6) unthresholded,
so a higher bar admits nothing on exactly the fully-escalated queries reuse would pay
for. Thin admission carries no risk because arm zero re-executes every hit against the
new query's tests (invariant #5): it costs a wasted verification, never a wrong
answer. A test now asserts a solve admits, and one asserts every shipped config can.

## Experiment 22 — §5.5(4), the §2.9 shift measured as a controlled dial ($0, offline)

**§2.9 is now tested rather than assumed, at n=409, for nothing.** The claim under
test: a warm cache absorbs the head of the query distribution, so the router observes
`D | x not in H_t` rather than `D`, and a certificate calibrated on `D` is void. That
had been argued from first principles for the whole study.

Experiment 20 established the direct test cannot be run on this benchmark —
absorption tops out at 2/488 (0.4%) — and concluded it needs absorption as a
*controlled dial*. That is `go-cascade absorption` (`internal/calibrate/absorb.go`).
The dial is applied to *recorded* observations, which is what makes the whole sweep
free: the n=409 records profile every tier on every problem, so any absorption pattern
crossed with any threshold vector replays offline with no model queries. 60 rows,
$0.00, seconds.

**Absorption is injected, not observed.** What this measures is the certificate's
sensitivity to a shift of *known shape and size* — precisely what §5.5(4) asks — and
precisely not a claim about how any real cache warms. Reading a number here as "the
observed effect of cache warmth" would repeat the error experiment 20 caught.

### The finding that reshaped the experiment: uniform absorption is a null

Issue #52's own framing was "inject exact duplicates at known rates." Run that way it
would have measured **nothing**, and reported the null as evidence about §2.9.
Dropping a random subset of an exchangeable sample leaves an exchangeable sample:

| ρ | n_res | acc(residual), uniform | acc(residual), easy-first |
|---|---|---|---|
| 0.00 | 409 | 0.7702 | 0.7702 |
| 0.20 | 327 | 0.7706 | 0.7125 |
| 0.40 | 245 | 0.7796 | 0.6163 |
| 0.60 | 164 | 0.7500 | 0.4268 |
| 0.80 | 82 | 0.7439 | **0.0000** |

Uniform absorption moves cheap-tier accuracy by noise, in both directions, at every
rate. §2.9's premise is a *head*-shaped filter — recurring queries are the easy ones —
and only a selective pattern produces the shift the section warns about. So
`AbsorbUniform` ships as an explicit **null control**, not as the main pattern, and
its spread is what the selective rows are read against.

### The uncorrected arm: the certificate goes optimistic, monotonically

Calibrating on the full recorded sample and deploying those thresholds on the residual
— the mistake §2.9 describes. The yardstick is the null control's envelope, and it has
to be measured over **seeds**, not read off one run: the uniform draw is random, so a
single sweep's max |gap| understates it. Over **10 seeds × 4 rates (40 draws)** the
uniform envelope is **max |gap| = 0.0389**, mean 0.0113. (The first version of this
write-up quoted 0.0267 from one seed, which moved the ρ=0.4 verdict — see below.)
The tool computes this itself — `-null-seeds` (default 10) — and writes it to the
`null_envelope` block of the JSON, precisely so nobody has to compare a treatment row
against one draw of the control. Underpowered and uncertified rows are excluded from
the envelope: their spread is sample-size noise, which would inflate the very bar it
is supposed to set.

| ρ | acc(res) | calibrated risk | deployed risk | gap | beyond the 40-draw null envelope? |
|---|---|---|---|---|---|
| 0.20 | 0.7125 | 0.0587 | 0.0734 | +0.0147 | no |
| 0.40 | 0.6163 | 0.0587 | 0.0939 | +0.0352 | **no** |
| 0.60 | 0.4268 | 0.0587 | 0.1341 | +0.0755 | yes |
| 0.80 | 0.0000 | 0.0587 | 0.2073 | **+0.1486** | yes |

At ρ=0.6 the certificate promises α=0.10 and delivers 0.134 — **an actually violated
bound**, not merely a loose one. `cheap-accept` (absorbing what the *policy* served,
the closest analogue to a cache warmed by a running cascade) reproduces it: +0.0117 →
+0.1364 over the same range.

**Read against the honest envelope, the effect is established at ρ ≥ 0.6, not ρ ≥ 0.4.**
The selective rows are exactly reproducible — `easy-first` is a sort, so its +0.0755
does not move across seeds at all — but the *control* they are compared against does
move, and it is the control that sets the bar. This is the second time in this study
that one draw was not a diagnosis.

### The corrected arm, and the cleanest row in the sweep

The tidiest comparison holds sample size **fixed** and varies only the shift. At
ε=1.0, ρ=0.2 every pattern calibrates on exactly 327 records:

| pattern | n_cal | acc(res) | risk | certifies |
|---|---|---|---|---|
| uniform | 327 | 0.7706 | 0.0612 | **yes, [1, 1]** |
| easy-first | 327 | 0.7125 | 0.0673 | no |
| cheap-accept | 327 | 0.7217 | 0.0642 | no |

Same n, same α, same δ, same grid — so sample size cannot explain the difference and
only the shift can. Shadow sampling drives the risk gap to **exactly zero** at ε=1 by
construction (calibration stream = deployment stream), and what remains is a *refusal
to certify*. That is the correction working: when the residual's true risk exceeds α,
the honest output is no certificate, not a smaller number. §2.9's correction does not
rescue the cost win under heavy absorption — it converts a silent violation into a
visible refusal, which is the whole point of having a certificate.

### The limit, stated rather than buried

Shadow sampling buys distributional correctness by **spending sample size**, and the
sweep's small-ε rows run out of it. Certifying α=0.10 at δ=0.10 needs **n ≥ 22 even
with zero observed errors** (`calibrate.MinCalibrationSize`: at r̂=0 the
Hoeffding term is exactly (1−α)^n, so the bound needs (1−α)^n ≤ δ). An ε=0.05 draw
off an 82-record residual is 4 records. Three rows fall under that floor and are
flagged `underpowered` in both the JSON and the printed table — their gaps are noise
about sample size, not evidence about the correction, and several are *negative*.
Without that column they would read as findings.

**What this does not license.** Invariant #8 stands. The result says the shift is real
and large under a head-shaped filter, and that calibrating behind a warm cache is
exactly as unsafe as §2.9 claims — the opposite of a reason to relax it. And because
absorption here is a dial rather than an observation, the *rates* are stipulated: what
is measured is the response curve, not where a real deployment sits on it.

Records: `results/absorption-n409.json` (60 rows plus a `null_envelope` block).
Reproduce with `go-cascade absorption -records
results/s55-fixed.records.execution.json -alpha 0.10 -pattern
uniform,easy-first,cheap-accept -epsilon 0,0.2,0.5,1.0`. The printed table's `sig`
column marks each selective row `*` or `-` against the envelope; the envelope's own
seeds are derived from `-seed`, so a published number is reproducible from the command
that printed it.

## Experiment 23 — §5.5(2) arm (e), self-consistency at matched cost ($0 so far, offline)

**Arm (e) is implemented, and the free design check rules it out at every tier but the
cheapest — before any spend.** Arms (a), (b) and (d) are recoverable from the profiled
records because every tier ran on every problem. Arm (e) is not: "matched cost" means a
sampling budget equal to the cascade's realized spend, and self-consistency votes over
candidates the cascade never drew. It needs its own sampling pass. What *is* recoverable
for $0 is whether the arm is well-posed at all.

Matched per problem, not on the mean — averaging would fund every problem alike and hide
that the expensive problems are exactly the degenerate ones. Under τ=[1,1] the budget is
**$0.0101/query**; dividing each tier's recorded cost by its profiled 2:1:1 fan-out gives
per-sample costs of $0.000174 / $0.003579 / $0.006155.

| tier | median fan-out (p10–p90) | below a 3-sample vote |
|---|---|---|
| small (Maverick) | **49** (38–81) | 0/409 (0.0%) |
| mid (Sonnet 4.5) | **2** (2–3) | 324/409 (**79.2%**) |
| large (Opus 4.5) | **1** (1–1) | 407/409 (**99.5%**) |

The cascade's whole spend is roughly one frontier call, because escalation to the frontier
tier is most of what the spend *is*. So a frontier-model arm (e) at matched cost is
**always-frontier relabelled** on 99.5% of problems, and a mid-tier one is a two-way vote
on 79% — and two samples either agree unanimously or tie, carrying nothing a single draw
does not. Run as §5.5(2) literally specifies it, with the tier unnamed, arm (e) would have
**reported a degenerate configuration as a null result about self-consistency.**

That is the same trap as experiment 22's uniform absorption, caught the same way: compute
what the design can exhibit before paying. So the feasibility report ships as a
first-class part of the arm rather than a footnote — `go-cascade selfconsistency` prints
it, **refuses `-sample` on a tier it has ruled out**, and costs nothing.

Only the cheap tier is well-posed, and there it is exactly §3.5's comparison: **49 votes
on how the code is written against 2 on what it does**, for the same money. The arm is
built to be a fair foil, not a strawman — the vote is over normalised source (comments,
formatting and import order do not split agreeing candidates) and reports **raw plurality
mass**, not the Wilson bound invariant #9 requires of the *routing* score, because arm (e)
crosses no threshold and bounding it below would handicap it for nothing. It never
consults the verifier to pick a winner; a text vote that peeked at execution would be
behavioural clustering with extra steps. Both selectors are scored on the **same
candidates at the same cost**, which isolates the selector rather than the budget, and a
cluster abstention (nothing survived the ladder) is reported separately rather than scored
wrong — it is a sound refutation of the whole sample (invariant #4) and an escalation in a
real cascade.

Records: `results/arm-e-feasibility-n409.json`. Reproduce with `go-cascade
selfconsistency -records results/s55-fixed.records.execution.json -config
examples/bench/config.go-specialist-211.json -tau 1,1 -provider=mock` (mock only builds
the router; nothing is queried). **The paid sampling pass is scoped at $4.12 — the matched
budget, by construction — and is unrun pending spend approval.**

## What is NOT established (open, honest)

- **Deployable-α certification was reached at n=64 and does NOT hold at n=409
  (experiment 21 supersedes 12–13 on this point).** Read the paragraph below as the
  n=64 story: at n=409 on MultiPL-E Go the lowest certifiable α is **0.084**, not 0.05,
  and the floor (empirical risk 0.0538) is **genuine model accuracy** — execution's
  oracle agreed with ground truth on 1096/1096 observations, so there is no noise left
  to blame. The n=64 result rested on **0/52 errors**, i.e. on a sample too small to
  contain the errors the model actually makes; at 6× the problems it makes them. The
  n=409 number is the trustworthy one, and the honest claim is **"α≈0.08 certifies,
  α=0.05 does not."** The cost tension below also reproduces at n=409 rather than
  dissolving (`[1,1]` below α=0.11, `[0.1,1]` and a 2.2× win at or above it).
- **(n=64 history, retained because the mechanism analysis is still correct.)**
  Experiment 8's "α=0.05 is unreachable; the floor is model accuracy
  (~0.11)" was an artifact of oracle noise. With the `-refs -pin-api` gate removing
  spec-model test noise, the completed n=64 pinned run has genuine model errors
  **0/52** and **certifies α=0.05** (valid=true, 0 empirical risk) — the first
  deployable-α certificate in the study, which the judge arm cannot match on the
  same candidates (its lowest empirical risk is 0.077). At 2:1:1 this *cost* the
  cost win (thresholds `[1,1]` → full escalation → pricier than frontier). Raising
  the cheap tier to 5 samples (experiment 13) lifted the unanimity ceiling
  0.425→0.649 and, **on one draw**, certified α=0.05 with τ0=0.4 while the cascade
  was 2.13× cheaper than frontier at 0 risk. But experiment 14 (a theorem + two
  repeat draws) established this **does not replicate**: fan-out provably buys
  discrimination headroom against *flaky* cheap-tier errors (gap widens with N) and
  *none* against *confident* ones (gap 0 ∀N), and across three fresh 5:1:1 draws only
  one got τ0<1.0 — another hit a cheap-tier confident-wrong answer (τ0→1.0, no cost
  win) and a third hit a top-tier miss (valid=false). **So the deployable-α cost win
  is achievable on a good draw, not guaranteed:** at n≈53 a single confident cheap
  error or top-tier miss moves the certificate. The binding constraint is model
  accuracy at small n; a *robust* win needs more calibration data or a more accurate
  cheap tier, not more fan-out. **Experiment 17 turned this into a frequency** — three
  more 5:1:1 draws *with* `-compare` (β=0), six draws total: **2 of 6 certify with a
  cost win, 3 of 6 certify-but-pricier (`[1,1]` collapse), 1 of 6 fails to certify.**
  And the mechanism is sharper — and more fragile — than "lucky draw": the win happens
  **iff the clean calibration set has zero confident-wrong tier-0 answers** (exact
  across all six), and *both* observed wins had confident-wrong answers that were merely
  **oracle-unsound-excluded** (Maverick's confident mistakes coincided with the spec
  model's unsound-test problems). So the win rides a coincidence between two independent
  error processes, not cheap-tier robustness. `scale_is_palindrome` is the recurring
  *sound-oracle* confident error behind the [1,1] collapses.
- **The scar-free-race blind spot is one data point and was not reproduced.**
  η_fa > 0 was observed exactly once (the pilot's model-authored race). The
  race-seeded test (experiment 6) caught all 20 seeded races — but only because
  sync-*deletion* leaves a visible scar; it does not produce the scar-free,
  self-consistent racy code that was actually false-accepted. The **scar-free race
  operator now exists** (`-seed-kind=scar-free-race`, issue #51): three operators
  that leave every sync call present and paired — `Lock`/`Unlock` → `RLock`/`RUnlock`
  around a write, moving the last guarded statement out past the `Unlock`, and
  `wg.Wait()` → `defer wg.Wait()`. All three are validated to compile and to be
  refuted under `-race`. The loop-variable-capture operator suggested earlier is
  **obsolete**: Go 1.22 made loop variables per-iteration, so that mutant no longer
  races (pinned by `TestLoopVarCaptureDoesNotRaceOnThisToolchain`). **The generator
  is built; the paid sweep comparing scar-free η_fa against the sync-deletion 20/20
  has not been run** — it needs the 11 concurrency problems of the hand-written set
  (issue #50), since MultiPL-E Go has none.
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
- **§2.9 is tested under an *injected* shift, not an observed one (experiments 20, 22).**
  The response curve is measured; where a real deployment sits on it is not. The direct test, §5.5(4), was scoped to
  run live and then measured offline instead: absorption on MultiPL-E Go tops out at
  **2/488 (0.4%)** against **95.1%** retrieval candidacy, because arm zero re-executes
  (invariant #5) and lexical similarity does not imply transferability here. A 0.4%
  cache induces no distribution shift, so the experiment has nothing to detect. **This
  is a statement about the benchmark, not a licence to relax invariant #8** — real
  traffic has genuine repeats; independently-sampled benchmarks are built to avoid them.
  Testing §2.9 needs **duplicate injection** (absorption as a dial) and then
  calibration drift as a function of it. **Now run — experiment 22**, offline for $0,
  and the shift is real: at ρ=0.6 under a head-shaped filter the certificate promises
  α=0.10 and delivers 0.134. What remains stipulated is the *rate*, since absorption
  there is a dial rather than an observation.
- **Judge β depends on the prompt.** The judge ran with "when in doubt, FAIL";
  its false-rejection counts would move under a different operating point. The
  strictness knob exists (`--judge-strictness`) but was only exercised on small n.
- **Small n in experiments 1–20** (n ≤ 64). Those counts have wide intervals; treat
  directions, not magnitudes, as the finding. **Experiment 21 is the exception** (n=409
  usable on a standard benchmark) and is where a magnitude can be quoted.
- **The `-race` rung is untested at scale.** MultiPL-E Go has **0** concurrency problems,
  so experiment 21 — the only large-n run — never exercised the ladder stage that caught
  the study's sole confirmed judge over-acceptance. The n=409 result therefore validates
  the *statistics* at scale while leaving the *most expensive verifier stage* covered only
  by the 64-problem hand-written set. That set is not obsolete.
- **η_fa's mechanism is still argued.** Experiment 21 measures 11 over-acceptances but
  the records store no candidate source, so the defect classes cannot be recovered.
  "The judge's blind spot is reading-invisible defects" needs a run that retains the
  accepted program for disagreeing observations — a disk cost, not a money cost.

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

Total live spend across all nineteen experiments plus offline diagnosis: roughly
$108–126 (the first eleven ~$65–69; experiments 12–17 ~$30–37; experiment 18 ~$5–7;
experiment 19 ~$8–14, **estimated not measured** — the `estimator` subcommand
records no cost field, and the figure includes ~$4–7 lost to a killed run whose
pre-atomic-write checkpoint was destroyed).
