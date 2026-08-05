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

**One-line summary:** across twenty-four live experiments (plus five decided offline for $0)
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
accuracy at small n. **The twenty-seventh experiment closes that thread with a bound
rather than another draw:** at n=409 the confident-wrong class is a measured **13/409 =
3.2%**, which *predicts* experiment 17's 2-of-6 frequency (0.181 against an exact CI of
[0.043, 0.777]) — so the cost win was small-sample sampling of a fixed rate, not
evidence about fan-out, and P(win) *falls* monotonically in n. And an **omniscient**
tier-0 gate — accept exactly the cheap answers execution says are correct, unattainable
by construction and therefore an upper bound on every fan-out, statistic and threshold
vector — is worth only **1.83×** against always-frontier. The cheap-tier lever is not
under-tuned; it is **exhausted**, and more than half the risk of the cheap policy is
incurred at the *frontier* tier, where no cheap-tier intervention reaches.

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
| 23 | [arm (e) feasibility](README.md#experiment-23--552-arm-e-self-consistency-at-matched-cost-0-so-far-offline) | §5.5(2)'s last unimplemented arm — self-consistency at matched cost. Asks first, for $0, what the matched budget actually buys | **the arm is implemented and the free check rules it out at every tier but the cheapest.** Matched budget $0.0101/q under τ=[1,1]; at the profiled 2:1:1 fan-out that buys median **49** cheap-tier samples (0.0% below a 3-vote), **2** mid (79.2% below), **1** frontier (**99.5%** below). So a frontier arm (e) at matched cost is **always-frontier relabelled** and a mid-tier one is a coin flip — run as §5.5(2) literally specifies it (tier unnamed) it would have **reported a degenerate configuration as a null about self-consistency**, the same trap as experiment 22's uniform absorption. Only the cheap tier is well-posed, and there it is exactly §3.5's contrast: **49 votes on how the code is written vs 2 on what it does**, same money. Built as a fair foil, not a strawman — normalised-source vote (formatting/comments/import order do not split it), **raw plurality mass** not the Wilson bound (invariant #9 governs the *routing* score; arm (e) crosses no threshold), and it never consults the verifier to pick a winner. Both selectors scored on the **same candidates at the same cost**, so the selector is isolated; cluster abstentions reported separately, not scored wrong (invariant #4). `selfconsistency` **refuses `-sample` on a ruled-out tier.** Paid pass **run — experiment 24** |
| 24 | [arm (e) live](arm-e-live-n409.md) | the paid pass at the one tier experiment 23 did **not** rule out: does a source-text majority vote beat behavioural clustering at matched per-problem cost, on identical candidates? | **execution wins decisively, and the interesting part is *where*.** Paired on the 366 rows both selectors answered: text vote **295/366 (0.806)** vs behavioural cluster **335/366 (0.915)**, median fan-out **50** samples — ~50 votes on how the code is *written* against 2 on what it *does*, same money, and the text vote still loses. McNemar on the 50 discordant pairs: **45 text-wrong/cluster-right vs 5 the other way, p = 4.2e-09**. **The finding is the abstentions:** where the cluster abstained (nothing survived the ladder — an escalation in a real cascade), the text vote was right **1/43 (0.023)** while its vote mass barely moved (0.604 vs 0.661 elsewhere) — it is **confidently wrong exactly where execution knows it has nothing**, which is an inversion, not the gradual degradation a "weaker selector" story predicts. Agreement carries no signal (313 agreed rows: 293 vs 290); the **entire** margin is the 53 disagreements (text 2, cluster 45), i.e. when they diverge it is almost always the text vote preferring a popular wrong program. **A reporting bug found and fixed in the same pass:** the summary printed text over all 409 voted rows against the cluster over 366, reading as a 0.19 gap where the paired figure is **0.11** — both numbers correct, different denominators, no test failing. Bill **$18.77** ($4.71 matched + $14.06 oracle = 75%), against the **$4.12** originally quoted, which was the matched *budget* mistaken for the invoice |
| 25 | [concurrency coverage](conc-coverage-n11-2026-08-04.md) | the `-race` rung — the ladder stage that caught the study's **only** confirmed judge over-acceptance — **never fired in experiment 21**, because MultiPL-E Go has 0 concurrency problems and a skipped stage scores `OK`. Exercise it on the 11 concurrency problems of the hand-written set (`concurrency.jsonl`; exactly 11 of 64 references trip the predicate) | **the rung fires (2.1 s at `-count=3` vs 605 ms plain), and the finding is the oracle's clock, not concurrency.** A generated `TestHInt64Overflow` on a counter that increments by **one** needs 20 s at a measured 215 M adds/s — sound assertion, **non-terminating test** — and it refuted the *reference* eleventh in a 30 s budget, flagging the record `OracleUnsound` with `timed_out` on all three tiers. **Without #63 this reads as η_fa = 3/26 = 0.115**, 11× experiment 21's 11/1096, from one test; the reference passes its own canonical suite in 673 ms, so it was never machine load. Excluding it: **η_fa = 0/23, all 6 disagreements over-rejections** (small 4, mid 2, large 0) — and #49's retention gives its first live data (6/6 carry source), showing the **mirror image** of §3.5: *correct* code (disjoint-index writes, mutex-merged local maps) that *reads* racy. By-product: **6 of these 11 problems were oracle-unsound in experiment 12 vs 6 of the other 53** — ~5×, so the class the paper most needs has the least trustworthy oracle. n=11 certifies nothing (α ≥ 0.19 required at δ=0.10); coverage, not a certificate. $0.8, 13 min |
| 26 | [coder-specialist tier 0](qwen-coder-tier0-n64-2026-08-04.md) | the cheap bottom tier is the **only** cost lever that has ever worked in this study (experiment 11, 3.2–3.4×); every other lever bought accuracy with money and lost. Qwen3-Coder 30B A3B is the sharpest remaining test because it is **cheaper than the incumbent** ($0.15/$0.60 per MTok vs Maverick's $0.24/$0.97), so a win cannot be reread as "we spent more" — and it is the first *coder specialist* at tier 0 | **the premise is refuted in direction: the specialist is *less* accurate at tier 0, 0.9149 → 0.8298 paired over 47** (McNemar 5-vs-1, exact p=0.22 — clear direction, not significant at this n). Escalations rose **3 → 7** at τ=[0.1,1] and cost at the cheapest threshold rose **2.1× at 2× the risk**; no threshold vector is both cheaper and no riskier. The per-sample saving is real and **irrelevant** — 1.42× cheaper per clean sample, but **tier 0 is 0.8% of the bill** ($0.027 of $3.5), exactly as the scope doc predicted the win would have to come from escalations. Two by-products outlive the arm. **Repair, not the per-token rate, dominates cheap-tier cost:** a **5.9× spread** around the median, and the scope doc's clean 4-problem probe was **1.9× optimistic**. And **the α difference has a frontier-tier cause, not a tier-0 one** — Qwen certifies α=0.10 where Maverick certifies 0.05, which invites "its cheap tier is weak," but Qwen's mid tier is **perfect** and its *large* tier misses one problem; the final tier has no threshold (invariant #6), so that floors risk at 1/47 and α=0.05 is unreachable **whatever tier 0 does**. **A tier-0 intervention at n≈50 can be decided by one frontier draw.** Both arms carry exactly one confident-wrong tier-0 answer, so neither reaches experiment 17's zero-confident-wrong condition; in both, that class is commoner among the oracle-unsound **exclusions** than the inclusions — experiment 17's coincidence, reproduced. Also: `conc_safe_counter`, experiment 25's non-terminating test and the one id unsound in **both** prior draws, is **sound here**; the two arms' unsound sets overlap on **3 of 17** ids ever flagged. $3.5 (74% unrecorded spec), 54 min |
| 27 | [fan-out ceiling](fanout-ceiling-n409.md) | the tier-0 fan-out is the **most-worked lever in the study** — experiments 9, 10, 12a, 13, 14 and 17 turned it across 5:2:1 / 1:1:1 / 2:1:1 / 5:1:1, six live draws at 5:1:1 alone — and it was left at "achievable on a good draw." Both the theorem (14) and the frequency (17) were measured at **n≈53**, which experiment 26 has since shown is a sample size where one observation moves the certificate. Re-ask it at n=409, and compute the bound the earlier runs did not | **the lever is exhausted, and a bound closes the thread where another draw could not.** An **omniscient** tier-0 gate — accept exactly the cheap answers execution says are correct, unattainable by construction and therefore an upper bound on every fan-out, every statistic and every threshold vector — is worth **1.83×** against always-frontier ($0.00337 vs $0.00616 at risk 0.0465 vs 0.0587). The realizable τ=[0.1,1] policy looks *better* at 2.12×, but only by carrying **more** risk (0.0733): it accepts 80 score-0 answers that are **all wrong**, so it is cheap precisely because it is wrong. At matched risk nothing reaches the bound. **Experiment 17's 2-of-6 was never about fan-out:** the confident-wrong class is a measured **13/409 = 0.0318**, and that rate predicts P(zero in 53) = **0.181** against an exact Clopper-Pearson CI of **[0.043, 0.777]** on 0.333 — consistent with sampling a *fixed* rate, so six more draws would re-estimate p, not move it, and P(win) **falls monotonically in n** (0.219 at 47 → 0.127 at 64 → ~0 at 409). The cost win was always a small-benchmark artifact. Two structural findings: the 287 correct and 13 wrong unanimous answers sit at score **exactly 0.424987** — numerically inseparable, experiment 14's theorem *measured* rather than inferred — and the gate has **three reachable settings** at 2:1:1, not the 121 the grid implies. Finally, **more than half the risk of the cheap policy is incurred at the frontier tier** (16 of 30 errors at τ=[0.1,1], on the problems tier 0 refused), which no cheap-tier intervention can reach — the same lesson experiment 26 paid $3.5 to learn. **$0, ~1 s, offline** |
| 28 | [scar-free race coverage](scarfree-coverage-n11.md) | issue #51's scar-free race operators (PR #56) seed the class §3.1 is actually about — racy code whose sync scaffolding is **intact and balanced**, which sync-deletion cannot produce (it leaves a WaitGroup with no `Wait`, and the judge scored **20/20** on those by spotting imbalance). The generator was merged; the sweep was never run. Ask the free question first: can the operators reach a benchmark, and would the sample answer anything? | **declined at 9 seeds against a bar of 10 registered before the harvest — and the first pass of this experiment was wrong twice, both times in its own favour.** 15 AST sites yield **9 seeds** (compile + a real ThreadSanitizer report), 7 of 11 problems, split **4 `defer-wait` / 4 downgrade / 1 escape**. Against the sync-deletion control **0/20** the critical value is **≥3 of 9**, a null bounds η_fa at **≤0.283**, and P(≥1 event) is **0.866** at η_fa=0.2. Nothing was spent, because the bar was set before the number was known — but this is **close, not clear**: for ~$3 an 87% shot at an existence proof on a claim resting on n=1 is not obviously a bad buy, and "both branches buy nothing" is too strong at n=9. **Retraction (a):** the `RWMutex` downgrade was reported "structurally dead — no `sync.RWMutex` in `examples/bench/`". It was an **operator bug**: it rewrote the call sites and left `var mu sync.Mutex`, on the reasoning in its own doc comment that "the build filter enforces the RWMutex rather than a type check". Deferring a type constraint to the compiler turns an unreachable operator into an empty result, and an empty result reads as a finding. Co-mutating the declaration gives it **4 seeds**, so "one operator carries the set" is retracted too. **Retraction (b):** the control was **0/17**, which is the *logic* arm over **6 different** problems; sync-deletion on these 11 is **0/20**. That alone moved the critical value from 2 to 3 of 6 — and 2/6 was the row labelled "cannot resolve". Same error shape as *one denominator per paired comparison*. **The suite was green on a false conclusion** because only AST sites were pinned, and 15 stayed correct the whole time the seed count was wrong; `TestScarFreeSeedCountOnTheConcurrencyBenchmark` now pins seeds per problem and per operator, and `TestConcurrencyRefIDsMatchTheBenchFile` pins the problem count (the old guard keyed on a hardcoded id list, so adding the 12th problem the write-up recommends left every assertion passing on the stale 11). Three cautions: the arm requires an actual **DATA RACE**, not merely a `-race` failure (`conc_safe_counter`'s mutant returns `got 0 want 400` with no race — counted as a seed by the first pass); **all 9 seeds are also refuted without `-race`**, which shows the rung is not load-bearing for them but does *not* show a reader would notice (recorded, never filtered — filtering one arm breaks comparability with the other); and **9 is the wrong population** — `ProfileSeeded` mutates a **tier-0 model draw**, never a reference, and the one live scar-free false acceptance was model-authored. Reopen in cost order: harvest from model draws, implement the deferred-form escape (`syncCall` calls `Lock(); defer Unlock()` "the dominant Go idiom" and the operator skips it), then widen the corpus — the first two immune to the tuning hazard. **§3.1's mechanism stays ARGUED.** **$0, ~20 s local, no model calls** |
| 29 | [deferred-escape operator](deferred-escape-n11.md) | experiment 28 declined a ~$3 seeded scar-free sweep at **9 seeds against a bar of 10 registered before the harvest**, and named three reopening routes in cost order. Run the two free ones — is the decline a fact about the benchmark or about the instrument? | **about the instrument: the bar is now MET at 10, and the bar did not move — the measurement did.** The escape operator only ever fired on an *explicit* `Unlock`, skipping every `Lock(); defer Unlock()` (the dominant Go idiom), so it had **1 site** on an 11-problem concurrency benchmark. The deferred form gives **16 sites / 10 seeds** (4 `defer-wait` / 4 downgrade / 1 escape / 1 escape-defer), 7 of 11 problems. Critical value **≥3 of 10**, a null bounds η_fa at **≤0.259**, P(≥1 event) **0.893** at η_fa=0.2. Enforced mechanically, not editorially: the seed test pinned at 9 **failed** when the operator landed, which is how the flip was noticed. **The bigger change is seed quality:** experiment 28's Result 3 was that *all* 9 seeds were also refuted without `-race`, so the sweep could only ever have measured whether a reader notices a deterministically-wrong program; **9 of 10** are now, and the new seed is the first that genuinely needs the rung. **Route 1 is closed:** `ProfileSeeded` mutates a tier-0 **model draw**, never a reference, and harvesting that population gives **8 raw / 6 unique / 5 from execution-correct bases** — 9 recorded rows are 7 unique programs (`conc_safe_counter` 3× byte-identically) and `ProfileSeeded` never checks base correctness, so a mutant of an already-wrong program would count as a seed. Not clearly above 10; report the range, not the flattering end. **Two shipped-operator defects, both self-flattering:** (a) **a formatting artifact is a scar** — `go/printer` lays a block out from its statements' *recorded source lines*, not slice order, so any reordering operator emits a blank line at the mutation site, and **`gofmt` does not remove it** (it preserves author blank lines). 3 of 17 sites carried the tell; now **0 of 16**. Scope the position-clearing to the statements that **moved** — clearing the whole block re-lays-out code the mutation never touched (52 printed lines → **49**: a collapsed three-line closure and a dropped blank line, i.e. a three-line scar replacing a one-line one) versus 52 → 52 scoped. (b) the deferred form **manufactured a DEADLOCK**: undeferring an `Unlock` only unlocks on the fall-through path, so a `return` in the covered region hangs — wrong class twice over, since a timeout's cause can be external and a `Lock` with a `return` before its `Unlock` is a *visible* imbalance. Found by **reading both mutants**, not by the DATA-RACE filter, which would have dropped the hang and reported one seed either way. A `return` inside a `func` literal is **not** an exit. **Nothing spent; clearing a bar makes the sweep fundable, not authorized.** **$0, ~24 s local, no model calls** |

| 30 | [the scar-free sweep, run](scarfree-sweep-n9.md) | experiment 29 cleared the bar experiment 28 registered, so the sweep became fundable; it was priced, authorized, and run. Does a judge that reads code miss races whose synchronization scaffolding is **intact**, at a higher rate than races that leave a visible imbalance? | **both arms are NULL — scar-free 0/9, sync-deletion 0/27, at all three strictness levels, Fisher p = 1.0.** The test asks whether the scar-free rate is *above* the deletion rate; both rates are zero, so **there is no gap to test** and §3.1's reading-invisible mechanism stays **ARGUED** on its one live event. This was the branch the pre-registered arithmetic gave the larger probability (critical value ≥3 of 9; P(≥1 event) 0.866 only at η_fa=0.2). **The realized denominators are the finding.** The scar-free arm drew **9 seeds from 5 problems**, not the instrument's 10 from 7, because `ProfileSeeded` mutates a **tier-0 model draw** and not a reference — flagged before launch, and the problem sets differ in *both* directions (`conc_safe_counter` yielded here and not on the references; `conc_fan_in_merge`/`first_success`/`parallel_histogram` the reverse). A near-equal total over 5 problems instead of 7 is more **concentrated**, and several mutants of one base are not independent draws, so the effective n is *below* 9 and the ≤0.283 bound is optimistic. **The control came back 0/27 from 8 problems, not the 0/20 on file** — same operator, same benchmark, same config, different draw. That vindicates re-running it in-session: citing July's 0/20 beside today's 0/9 would have crossed a session boundary and model versions inside a two-arm comparison, the same error shape as *one denominator per paired comparison*, and invisibly, since both are real results here. The verdict happens to be unchanged (critical value is ≥3 against both), which is luck, not robustness, and only checkable because the control was re-measured. **The one surviving asymmetry is bound tightness, not observed rate:** 9 seeds bound the scar-free class at ≤0.283 where 27 bound the control at ≤0.105 — overlapping intervals, so not a mechanism, but the class §3.1 is about is the one we can least afford to sample, which is a fact about **operator reach**, not the judge. Validity check that did hold: `ScarFreeRaceKilledMutants` applies the `DataRace` filter on top of the shared harvest, so all 9 carried a real ThreadSanitizer report — without it this would be a null about deterministically-wrong programs in race clothing. **Gap found:** `-seed-kind` writes **no records, only stdout**, so per-problem counts and mutant sources are unrecoverable and the run is not resumable; on a null that costs little, on a positive event it would have destroyed the only interesting follow-up. `KilledMutant` already carries `Source`/`Desc`/`DataRace`/`PlainRefuted`, so it is a persistence path, not new instrumentation. Do **not** read the zeros as η_fa = 0. **~32 min, priced ex ante at ~$1.20 (ceiling $1.94) — cheaper than the decline's ~$3 because the seeded path skips the cascade tier loop entirely** |

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

### And the lever itself is now bounded — experiment 27

Both levers above are ways of making tier 0 *better*. Experiment 27 asks how much that
can be worth at all, and the answer caps the whole line of work: an **omniscient** tier-0
gate is worth **1.83×**. Since it is unattainable by construction (it needs the answer to
decide), it bounds every fan-out setting, every routing statistic, and every threshold
vector — including the 5:1:1 configuration six live draws were spent on.

That also reinterprets those draws. Experiment 17's "2 of 6 certify with a cost win" was
read as a property of fan-out; at n=409 the confident-wrong class is a measured
**13/409 = 0.0318**, and that single rate predicts P(zero in a clean set of 53) =
**0.181**, inside the exact Clopper-Pearson CI **[0.043, 0.777]** on 0.333. So the six
draws were sampling a fixed rate. Worse for the lever, P(win) *falls monotonically in n*
— 0.219 at n=47, 0.127 at 64, ~0 at 409 — so the cost win was a small-benchmark artifact
that gets rarer precisely as the evidence gets more trustworthy.

**Where cost work should go instead.** At τ=[0.1,1], 16 of the 30 errors are incurred at
the *frontier* tier, on problems tier 0 refused; the final tier has no threshold
(invariant #6), so no routing decision touches them. And the shared spec/oracle call is
74–91% of every bill while no routing decision touches it either. Tier 0 is the part of
the system that has been optimized hardest and matters least.

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
5. **Behavioural clustering beats a source-text majority vote at matched cost**
   (experiment 24, §5.5(2) arm (e)). Paired on identical candidates at the same
   per-problem spend: **0.915 vs 0.806** over 366 rows, McNemar **45 vs 5**
   discordant, p = 4.2e-09. The vote had ~50 samples to the cluster's 2 and still
   lost. **Where** it lost is the mechanism: on the 43 rows the cluster abstained,
   the text vote was right **1/43** while its confidence barely moved — it is
   confidently wrong precisely where execution reports having nothing. Agreement
   between the two selectors carries no signal (293 vs 290 of 313); the entire
   margin is the 53 disagreements (2 vs 45).

Together these make the paper's claim **more precise**: the executable oracle
both certifies lower *and* covers the judge's blind spot (reading-invisible
defects — scar-free races, and by extension aliasing / spec misreads). The
certification advantage on this stream is driven more by the judge's
over-rejection than by its rare over-acceptance — a mechanism the paper
underweights, but one that still favours the executable oracle.

Point 5 closes §5.5(2): every arm (a)-(e) has now been run, and the one that had to
be *sampled* rather than replayed is also the one that most directly tests §3.5's
contrast between agreement and behaviour. Its result is an inversion rather than a
gradient, which is stronger than the paper argues for.

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
the router; nothing is queried).

**The paid sampling pass is run — see [experiment 24](arm-e-live-n409.md).** Its approved
cost was wrong the first time: I quoted **$4.12**, which is the *matched sampling budget*, a
quantity derived from the records, as if it were the invoice. The real bill was **$18.77**,
because arm (e) also regenerates each problem's pinned spec and **no `Record` has ever
stored the spec cost** (see the section below). Both numbers now print separately, along
with `OverBudgetUSD`.

A 6-problem live smoke test before the full pass paid for itself three times over, and none
of the three defects it found were reachable from the mock (which reports zero cost and pins
no APIs):

1. **The paid path scored arm (e) against an unpinned oracle.** `cmdSelfConsistency` never
   called `SetPinnedAPIs`, while `SelfConsistency` reads `r.pinnedAPI[id]`. Arm (e) would
   have been compared against arm (b) *on a different oracle* — a differently-named,
   never-soundness-checked generated suite (invariant #4's corollary). `-refs` is now
   **required** for `-sample`.
2. **Spend exceeded the matched budget on 5/5 rows (+1.7% to +17.8%), silently.** A short
   probe both buys a larger fan-out and underprices it, so the error compounds upward.
   Now recorded as `OverBudgetUSD` and totalled in the summary. (My first diagnosis of this
   was wrong — I claimed a probe double-count and rewrote the arithmetic before checking
   that `floor(B/p)` and `1+floor((B-p)/p)` are identical. They are.)
3. **Spec cost discarded**, as above.

## Experiment 25 — concurrency coverage: the `-race` rung, and a generated test that cannot terminate ($0.8)

Full write-up: [`conc-coverage-n11-2026-08-04.md`](conc-coverage-n11-2026-08-04.md).

MultiPL-E Go has **0** concurrency problems, so experiment 21's n=409 run skipped the
`-race` rung on all 488 records — the rung that caught the study's only confirmed judge
over-acceptance never fired at the only large n, and a skipped stage scores `OK`, so
nothing said so. `examples/bench/concurrency.jsonl` is the coverage set: **exactly 11 of
the 64** hand-written references trip `UsesConcurrency`, now asserted in both directions
by `TestConcurrencyBenchActuallyReachesTheRaceRung` (every id trips it; no concurrency
reference is absent from the file).

**n=11 is coverage, not a certificate.** At δ=0.10, `MinCalibrationSize` needs α ≥ 0.19
for n=11 even at zero errors, so `valid=false` on both arms at the run's α=0.05 is
arithmetic about sample size, not a result about either oracle.

**The finding is the oracle's clock, not concurrency.** One of 11 records was excluded
`OracleUnsound` because the *reference* was refuted at `VA:accept` — with `timed_out`
set on all three tiers. The reference passes its own canonical suite in 673 ms, so it
was not machine load, which is precisely what the printed WARNING invites you to assume.
Reproduced deterministically for $0: `ConcurrentCount` increments by **one**, so
"overflow an int64 counter" taken literally is ~2^63 atomic adds. At a measured 215 M/s,
the generated `ConcurrentCount(2, MaxInt32)` needs **20 s** — inside a 30 s budget
already spent on ten prior tests. The assertion is correct and the test is sound; it
simply **cannot finish**. That is a third generated-oracle failure mode, alongside
experiment 19's over-rejections: neither wrong nor strict but **non-terminating**, and
indistinguishable from a slow host without `TimedOut`.

**Without #63 this run would have reported η_fa = 3/26 = 0.115**, against experiment
21's 11/1096 = 0.0100 — one non-terminating test, tripling three tiers because the truth
column is shared, producing the study's largest η_fa by an order of magnitude on the
exact defect class the mechanism argument is about. #63 shipped hours earlier as pure
instrumentation and changed a headline on first live use. Two of its design choices
earned their keep: the tally is over **records** (`1/11`, one suspect problem, not three
events), and the record is **kept** — it is the oracle-soundness gate that excludes it,
never the timeout flag (invariant #8).

With that record excluded, **η_fa = 0/23 and all 6 disagreements are over-rejections**
(small 4, mid 2, large 0). This is `classify_disagreements.py`'s first live data (#49),
and retention worked: **6/6 carry source**. Reading them shows the **mirror image** of
§3.5's claim — every one is *correct* concurrent code that *reads* as racy:
disjoint-index writes (`out[start+j]`, `j%workers == workerID`) into a shared slice,
per-goroutine local maps merged under a mutex, `context.WithCancel` first-writer-wins.
The judge has no execution, and disjointness is exactly the property text does not show.
Same blind spot as the over-acceptance case, running the safe direction. It does **not**
show the judge is safe on concurrency: the scar-free race class that produced the pilot's
one over-acceptance was never sampled here (issue #51).

By-product, and the strongest justification for keeping this set: experiment 12 ran
the same config over all 64, so its 12 unsound records split **6 of the 11 concurrency
problems against 6 of the other 53** — concurrency problems produce an unsound generated
oracle **~5× as often**. The class the paper most needs is the class whose oracle is least
often trustworthy, so usable n shrinks fastest where it is scarcest. And the two draws
share **exactly one** unsound id (the non-terminating one, the only deterministic defect
among them), reproducing experiment 19's warning that rejection-side rates are unstable
at this n.

## A correction to every cost figure on this page (no experiment, no cost)

**Every per-run cost quoted above understates the bill, by roughly 4× in aggregate, in one
direction, for one reason.** Cost Explorer for 2026-07-24…08-03 shows **~$197** of real
go-cascade Bedrock spend. The runs on this page total roughly **$40**.

| model | role | spend |
|---|---|---|
| `Claude4.6Sonnet` (incl. cache r/w) | spec / oracle / judge | **$154.89** |
| `Claude4.5Opus` | large tier | $23.48 |
| `Claude4.5Sonnet` | mid tier | $14.31 |
| `Claude4.5Haiku` | earlier small tier | $3.03 |
| Llama 4 Maverick | tier 0 | $1.14 |

**The oracle model is 91% of the bill and appears in no record.** Every `r.spec(...)` caller
in `internal/cascade` passes a throwaway `&Result{}` — `profile.go`, `judge.go` (×4),
`estimator.go`, `selfconsistency.go` — so the shared oracle's cost has never been written to
a `Record`. Tier costs, the only thing recorded, are the *small* part of every run. Measured
live: **$0.0408/problem** for a pinned spec (~2,700 output tokens — two Go test partitions
at sonnet-4-6's $15/MTok out).

Concretely: experiment 21 reports **$8.15** for 488 problems against only $5.14 of summed
`tiers[].cost_usd`; specs alone were ~$20, so its true cost was **~$25–30**. The
*ratios* on this page are unaffected — the spec term is identical across arms and cancels in
every paired comparison, which is why it went unnoticed — but no absolute figure here is the
invoice.

### And every *policy* ratio on this page is a routing ratio

The cancellation above is exactly why the oracle result is safe and the **cost** result is
not. A paired comparison between two *oracles* on identical candidates is unaffected by a
term both arms pay. A comparison between two *policies* — cascade vs always-frontier — is
a comparison of the term that the shared oracle dwarfs:

| policy (n=409) | routing | + oracle | total | vs frontier |
|---|---|---|---|---|
| always-frontier | $0.00616 | $0.0408 | $0.04696 | 1.00× |
| cascade τ=[0.1,1] | $0.00290 | $0.0408 | $0.04370 | **1.07×** |
| cascade τ=[1,1] (certifies at α≤0.10) | $0.01008 | $0.0408 | $0.05088 | **0.92×** |

```bash
python3 results/total_cost.py results/s55-fixed.records.execution.json
python3 results/total_cost.py -spec 0.01 results/s55-fixed.records.execution.json  # sensitivity
```

The oracle price is a **flag, not a constant**, because $0.0408 is a single live
measurement and every total-cost ratio is sensitive to it. Reconcile it against a bill
line before quoting one.

**The 2.12× routing win is 1.07× in total, and the policy that certifies at a deployable α
is 0.92× — slightly *pricier* than always-frontier.** The collapse is structural, not an
artifact of these particular numbers: for a shared cost *S* and frontier routing cost *f*,
a routing ratio *R* becomes (S+f)/(S+f/R), which tends to 1 as S/f grows **whatever R is**.
At S/f ≈ 6.6 no achievable *R* clears ~1.2× — not experiment 26's 9.0× measured at n=64,
and not experiment 27's omniscient bound (1.83× routing → **1.06× total**).

Three consequences, and they set the direction for any further cost work:

1. **Quote total cost, or say "routing" explicitly.** Every ratio in the table above this
   section is routing-only. That is the right denominator for asking "does the router
   route well" and the wrong one for "is this cheaper to operate."
2. **A lever confined to the tiers can move at most ~13% of the bill.** Tier 0 specifically
   is 0.8% (experiment 26). This is the arithmetic behind experiment 27's conclusion that
   the cheap-tier lever is exhausted — even the unattainable bound is worth 6% total.
3. **The only cost term large enough to matter is the oracle itself**, and break-even is
   steep: a 1.5× *total* win needs the oracle ~11× cheaper, and 2× needs ~114×. A cheaper
   spec model is the obvious candidate and runs straight into this study's own findings —
   a weaker test author writes buggier suites, and experiment 19 showed generated-oracle
   errors are **over-rejections**, which cost escalations rather than risk and are
   therefore *invisible to the certificate*. That makes it a measurable trade-off
   (spec cost vs `OracleUnsound` rate vs escalation rate) under invariant #3 as a hard
   constraint, not a free win.

Two traps in reading it back, each of which produced a wrong answer first:

1. **Claude bills under service `Amazon Bedrock Service`, not `Amazon Bedrock`.** Filtering
   on `Amazon Bedrock` returns only the open-weight models and totals $1.14; I briefly
   concluded the whole study cost a dollar. Query both service names.
2. **That service line also contains Claude Code's own usage**, which dwarfs the study
   (~$11K over the same window). Filter by `USAGE_TYPE` model substring. The naming differs
   between families (`Claude4.6Sonnet` vs `anthropic.claude-opus-5`), so one regex over both
   mis-buckets.

**How to quote a future run:** reconcile against a known past *bill*, not a derived total,
and multiply the spec term in explicitly — it dominates. A spec cache does not recover it:
each spec keys on `cache.ProblemHash(problem + pinned + api)`, so across distinct problems
reuse is exactly zero, and calibration forces `cache_dir=""` anyway (invariant #8). Check a
cache key's cardinality before claiming a cache saves anything.

## Experiment 26 — a coder-specialist tier 0 is not a cost lever ($3.5)

Write-up [`qwen-coder-tier0-n64-2026-08-04.md`](qwen-coder-tier0-n64-2026-08-04.md); scope
in [`qwen-coder-tier0-scope.md`](qwen-coder-tier0-scope.md); config
`examples/bench/config.qwen-coder-211.json`.

The cheap bottom tier is the only cost lever in this study that has ever worked
(experiment 11, 3.2–3.4×). Every other lever bought accuracy with money and lost — an Opus
planner was 3.1× *pricier* (15), a Haiku planner did not rescue it (16), plan-once-reuse was
negative (18). Qwen3-Coder 30B A3B was the sharpest remaining test because it is **cheaper
than the incumbent, not merely different** — verified us-west-2 on-demand rates $0.15/$0.60
per MTok against Maverick's $0.24/$0.97 — so a win could not be reread as "we spent more."
It is also the first *coder specialist* at tier 0; every prior cheap tier was general.

**The premise is refuted in direction. A coder specialist did not raise cheap-tier accuracy,
it lowered it: 0.9149 → 0.8298 paired over the 47 problems usable in both arms** (McNemar
5-vs-1, exact p=0.22 — directionally clear, not significant at this n). Escalations rose
3 → 7 at τ=[0.1,1] and cost at the cheapest threshold rose **2.1× at 2× the risk**. There is
no threshold vector on this draw where the specialist is both cheaper and no riskier.

**The per-sample saving is real and irrelevant**, which is the scope doc's own prediction
confirmed: 1.42× cheaper per clean sample, and **tier 0 is 0.8% of the bill** ($0.027 of
$3.5), so a cheaper tier-0 line cannot pay for itself. Two by-products worth keeping:

- **Repair, not the per-token rate, dominates cheap-tier cost.** Tier-0 cost per problem
  spans $0.000182–$0.002023, a **5.9× spread** around the median, entirely repair rounds.
  The scope doc's 4-problem clean probe ($0.000092/sample) was **1.9× optimistic** against
  the measured median. Price a tier with `repair_depth` set as configured, not from a clean
  generation.
- **The α difference has a frontier-tier cause, not a tier-0 one — and reporting it the
  obvious way would have been wrong.** Qwen certifies α=0.10 where Maverick certifies 0.05,
  which invites "its cheap tier is too weak." Per-tier misses say otherwise: Qwen's mid tier
  is **perfect (0 misses)** and its *large* tier misses one problem
  (`hard_num_mean_overflow`). The final tier has no threshold (invariant #6), so a frontier
  miss floors empirical risk at 1/47 = 0.0213 and α=0.05 is unreachable **whatever tier 0
  does**. Both arms share the mid and large models; it is the same problem Maverick's *mid*
  tier missed. So **a tier-0 intervention at n≈50 can be decided by a single frontier
  draw** — exactly the failure mode experiment 17 saw in 1 of its 6 draws. Read future
  cheap-tier arms at α≥0.10, or at an n where one top-tier sample cannot move the
  certificate.

Both arms carry exactly one confident-wrong tier-0 answer in the clean set, so neither
reaches experiment 17's zero-confident-wrong condition and neither gets the clean win. In
both arms the class is **more common among the oracle-unsound exclusions than the inclusions**
(3-of-12 and 5-of-8) — experiment 17's coincidence, reproduced: the cheap tier's confident
mistakes keep landing on problems whose generated oracle is independently unsound, and the
exclusion set is not the model's doing.

Two infrastructure findings from the scoping, both verified live rather than assumed:

- **There is no second endpoint.** `qwen.qwen3-coder-30b-a3b-v1:0`,
  `qwen.qwen3-coder-480b-a35b-v1:0`, `moonshotai.kimi-k2.5`, `zai.glm-4.7` and
  `mistral.devstral-2-123b` all answer through plain `bedrock-runtime converse` — the path
  `internal/model/bedrock.go` already uses. The arm is a config file, not a provider.
- **`go-cascade models` will not list any of them**, which is what made me report the
  opposite at first. It calls `ListInferenceProfiles`, which returns only `us.*` profiles;
  the open-weight IDs are bare. Use `aws bedrock list-foundation-models --region us-west-2`.

And two defects in the scope doc itself, caught before spending — each would have produced a
wrong number:

1. **`-refs examples/bench/refs` resolves 28 of 64 references**, not 64: `hard/` and `scale/`
   carry their own `refs/` subdirectories. That leaves **36 problems with no
   oracle-soundness gate at all** (`validateOracle` returns `OracleSound` on a missing
   reference — correct behaviour, and exactly what makes the mistake silent) and is not
   comparable to the matched Maverick arm. The `loaded 64/64` stderr line is the check.
2. **Maverick's "$0.12/MTok in" is its *batch* rate**; Converse bills on-demand at $0.24.
   The config has always used 0.24, so no published figure moves, but the error ran in the
   conservative direction — Qwen is cheaper on **both** legs, not only output.
   `list-foundation-models` returns four rows per Meta model, and batch/on-demand differ by
   exactly 2×. Filter on `usagetype`.

Denominators, because the arms share nothing — not candidates, not generated tests, and **not
the exclusion set** (oracle soundness is a property of the *generated* suite, regenerated per
run): Maverick 52 usable of 64, Qwen **56** of 64, **47 paired**. The per-arm tier-0 rates
(0.8846 over 52 vs 0.8393 over 56) are each correct and **not comparable** — their 0.045 gap
is half the paired 0.085. `results/compare_tier0.py` computes every comparative figure over
the intersection with McNemar's exact test, prints per-arm rates only under a NOT-comparable
header, and with `-pair-out` hands the paired subsets to the shipped
`calibrate -from-records` rather than reimplementing LTT (a second untested copy of the
fixed-sequence ordering is how invariant #7 gets quietly broken).

**Instability, third data point.** `conc_safe_counter` — the non-terminating
`TestHInt64Overflow` that was experiment 25's entire headline, and the one id flagged unsound
in both prior draws — came out **sound here**; the spec model did not write that test this
draw. The two arms' unsound sets overlap on **3 of the 17 ids ever flagged**. Experiment 19's
"rejection-side rates are not stable at n=64" now reproduces across three subcommands and two
oracles.

Bill **$3.5** ($0.90 recorded tier cost + ~$2.61 unrecorded spec/oracle = **74%**) against
~$3.6 scoped. 64/64, 54 min, no kill.

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
  is built, the sweep was DECLINED at 9 seeds against a bar of 10 registered before the
  harvest, and a FOURTH operator has since moved the count to 10, so it now clears that
  bar** — see experiments 28 (`scarfree-coverage-n11.md`) and 29
  (`deferred-escape-n11.md`). The escape operator only ever fired on an *explicit*
  `Unlock`, skipping every `Lock(); defer Unlock()` — the dominant Go idiom — so the 11
  concurrency problems yield **16 AST sites and 10 seeds** (compile + a real
  ThreadSanitizer report), split 4 `defer-wait` / 4 downgrade / 1 escape / 1
  escape-defer. Against the sync-deletion control **0/20** the critical value is ≥3 of 10
  and a null bounds η_fa at **≤0.259**, with P(≥1 event) = 0.893 at η_fa=0.2. **The bar is
  met at the margin and nothing has been spent:** clearing a registered bar makes the
  sweep fundable, not authorized, and n=10 buys an existence proof rather than a rate. The
  deferred form needs a control-flow **veto** the plain form does not — undeferring an
  `Unlock` only unlocks on the fall-through path, so a `return` in the covered region
  DEADLOCKS instead of racing, which is both the wrong defect class and a *visible* lock
  imbalance (the deletion arm's territory). Two shipped-operator defects were fixed on the
  way, both self-flattering: a **formatting scar** (`go/printer` lays a block out from its
  statements' recorded source lines, so any reordering operator emits a blank line at the
  mutation site — and `gofmt` does not remove it; 3 of 17 sites carried the tell, now 0 of
  16), and the missing deadlock veto. **That write-up's first pass reported 6
  seeds against 0/17 and both figures are retracted:** the `RWMutex` downgrade was called
  "structurally dead" when it was an operator bug (it left the declaration a `sync.Mutex`,
  so every mutant failed to build; co-mutating it yields 4 seeds), and 0/17 is the *logic*
  arm over 6 **different** problems rather than sync-deletion on these 11. Both errors
  favoured declining. Note also that
  experiment 25's 6 judge disagreements were all over-rejections of *correct* code, so a
  scar-free sweep would measure against a judge already shown to mis-read this class in
  the safe direction. **§3.1's reading-invisible mechanism therefore remains argued, and
  the blocker is the corpus.**
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
- **Small n in experiments 1–20, and in 25–26** (n ≤ 64). Those counts have wide intervals;
  treat directions, not magnitudes, as the finding. **Experiment 21 is the exception** (n=409
  usable on a standard benchmark) and is where a magnitude can be quoted. Experiment 26 shows
  how sharp this bites for *interventions*: a tier-0 model swap at n=47 paired had its
  certificate decided by **one frontier-tier miss**, a tier the intervention did not touch.
  Read a cheap-tier arm at α ≥ 0.10, where a single top-tier error is inside the budget, or
  at an n where one such sample cannot move the bound.
- **No cheap-tier model swap has ever produced a cost win, and the one that did is
  unexplained.** Experiment 11's 3.2–3.4× (Llama 4 Maverick at 2:1:1) is still the only
  working cost lever in the study, and experiment 26 shows it is **not** attributable to
  model quality: a purpose-built coder specialist that is cheaper per sample was *less*
  accurate at tier 0 (0.9149 → 0.8298 paired) and escalated more. So what makes Maverick
  work at that tier is unidentified — it is not "a better cheap coder," because a better
  cheap coder was tried.
- **The cheap-tier lever is bounded at 1.83×, so the remaining headroom is small and
  the search for it should stop** (experiment 27). An *omniscient* tier-0 gate — accept
  exactly the cheap answers execution says are correct — is unattainable by construction
  and therefore upper-bounds every fan-out, statistic and threshold vector; it is worth
  1.83× against always-frontier at n=409. The realizable τ=[0.1,1] policy appears better
  (2.12×) only because it carries more risk: it accepts 80 score-0 answers that are all
  wrong. At matched risk nothing reaches the bound. This also retires the fan-out thread:
  the confident-wrong rate is 13/409 = 0.0318, and that rate *predicts* experiment 17's
  2-of-6 frequency, so those six draws were sampling a fixed rate rather than measuring
  a fan-out effect — and P(win) falls monotonically in n. **Any future cost work should
  target the frontier tier or the shared oracle, not tier 0**: more than half the risk of
  the cheap policy is incurred at the frontier (16 of 30 errors at τ=[0.1,1]), and the
  spec/oracle call is 74–91% of every bill.
- **The `-race` rung is exercised, but only at n=11** (experiment 25). MultiPL-E Go has
  **0** concurrency problems, so experiment 21 — the only large-n run — never ran the
  ladder stage that caught the study's sole confirmed judge over-acceptance. The n=409
  result therefore validates the *statistics* at scale while the *most expensive verifier
  stage* is covered only by the 11 concurrency problems of the hand-written set. That set
  is not obsolete, and it is scarcer than it looks: **6 of those 11 problems yielded an
  unsound generated oracle in experiment 12, against 6 of the other 53** — the class the
  paper most needs has the least trustworthy oracle, so usable n shrinks fastest exactly
  where it is scarcest. `-race` at large n needs a concurrency benchmark that does not
  yet exist.
- **η_fa's mechanism is confirmed in the *safe* direction only.** Experiment 21 measured
  11 over-acceptances but stored no candidate source, so its defect classes are
  unrecoverable. Retention now works (`TierObs.DisagreementSource`, #49) and experiment
  25 is its first live data: 6/6 disagreements carry source, and reading them shows the
  **mirror image** of the claim — correct concurrent code (disjoint-index writes, mutex-
  merged local maps) that *reads* as racy and is over-*rejected*. That confirms the
  mechanism — the judge cannot see disjointness because text does not show it — on the
  cheap side. **The dangerous side is still argued, and experiment 30 spent money
  establishing that it stays argued.** The sweep pairing the scar-free class against the
  sync-deletion arm was **declined** at experiment 28's 9 usable seeds against a bar of 10
  registered before the harvest; experiment 29 raised the measurement to **10** by
  implementing the deferred-form escape operator, so it cleared the bar without the bar
  moving; and experiment 30 **ran it. Both arms are null: scar-free 0/9, sync-deletion
  0/27, at all three strictness levels, Fisher p = 1.0.** The test the sweep exists to
  perform asks whether the scar-free rate is *above* the deletion rate, and both rates are
  zero, so there is no gap to test. The realized denominators are the story: the scar-free
  arm drew **9 from 5 problems**, not the instrument's 10 from 7, because `ProfileSeeded`
  mutates a **tier-0 model draw** rather than a reference — and the control drew **0/27
  from 8**, not the 0/20 on file from 2026-07-25, which is why it was re-run in-session
  rather than cited across one. The one surviving asymmetry is bound tightness, not
  observed rate: 9 seeds bound the scar-free class at ≤0.283 where 27 bound the control at
  ≤0.105, so the class §3.1 is about is the one we can least afford to sample — a fact
  about the operators' reach, not the judge. The route that looked cheapest — harvest from
  the model-draw population — is **measured and closed**: 8 seeds raw, **6** unique
  (`conc_safe_counter` recurs 3× byte-identically), **5** from execution-correct bases.
  Widening the corpus is the only route left, and it carries the tuning hazard.

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

**Do not read a study total off this page.** The per-run figures above are summed tier
costs, which omit the shared spec/oracle term entirely — Cost Explorer shows **~$197**
of real Bedrock spend against roughly $40 published, with the oracle model alone at 91%.
See [§A correction to every cost figure on this page](#a-correction-to-every-cost-figure-on-this-page-no-experiment-no-cost)
for the breakdown and for the two traps in querying it back. Recent runs quote both terms:
experiment 24 billed $18.77 against a $4.12 *budget*, experiment 25 cost $0.34 recorded
plus ~$0.45 unrecorded spec ≈ $0.8, and experiment 26 cost $0.90 recorded plus ~$2.61 spec
≈ $3.5 (spec **74%**) against ~$3.6 scoped — the first run scoped with the spec term
included up front, and it landed.

Experiment 26 adds a second lesson about pricing a *tier* rather than a run: its scope doc
priced Qwen3-Coder from a 4-problem probe at $0.000092/sample and the run measured
$0.000172 (median, clean) — **1.9× optimistic**, because the probe generated once while
`repair_depth: 2` makes a failing candidate cost extra turns. Tier-0 cost per problem spans
a **5.9×** range around the median, all of it repair. Price a tier at its median with
repair enabled.
