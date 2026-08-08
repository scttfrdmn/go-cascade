# Design space: combining models for reliable cost savings

2026-08-08. Six independent design explorations, run in parallel against this
repo's actual code and invariants (not generic proposals), followed by four
cross-cutting reviews looking for synergies and tensions across all six. This
is **design research, not a result** — no code changes to `internal/` have
been made. Two of the six explorations' own gating checks *have* since been
run and broadened (`results/design_space_offline_checks.py`, see the
"Recommended build order" section) — those are real $0 measurements against
existing records, but still first-pass signals bounding whether a
corresponding code change is worth writing, not findings on the order of the
numbered experiments in `results/`.

Grounding used by every exploration: `CLAUDE.md`'s nine numbered invariants,
`docs/verification-saturated-cascades.md`, and `CLAUDE.md`'s "Known gaps"
section, which already names several of these as unimplemented (non-nested
tier ordering §2.6, adaptive conformal inference, single-file-only scope).

## 1. Adaptive / non-fixed escalation ordering

**Mechanism.** The fixed cost-ordered ladder (cheap → mid → frontier) is
implemented by `sequential` walking `r.cfg.Tiers` in array order
(`internal/cascade/cascade.go:374-467`); escalation on failure is an
unconditional `continue` to `k+1` (lines 421-430). `preferRepair`
(`cascade.go:922-945`) is already a precedent for the right *shape* of change:
a marginal cost/benefit decision function using fixed (uncalibrated) priors,
not a loop rewrite.

Two very different-cost instantiations:

- **Phase 1 — stopping-time rule.** Decide escalate-vs-stop at the current
  tier using only that tier's own visible-partition signal (cluster score,
  cost-so-far, verifier diagnostics). No array reordering, no change to which
  tier opens next.
- **Phase 2 — true reordering (Pandora's Box).** Choose *which* tier opens
  next based on observed evidence. This requires re-keying
  `Certificate.Thresholds` and `calibrate.Replay`/`Risk` from positional
  tier-array index to tier *identity*, and relaxing `config.Validate`'s
  ascending-cost-order check (`config.go:232-237`).

**Invariant fit.**
- **#7** (fixed-sequence grid ordering, data-independent) is orthogonal in
  kind to online tier-opening order but coupled through calibration cost: a
  richer adaptive-policy parameter space is a bigger LTT grid, more expensive
  to calibrate at a given (α, δ, n).
- **#2** (never shop against the holdout) is the sharp risk. The rule must be
  restricted to visible-partition signal only, exactly like `preferRepair`
  already is. A subtler failure: reordering among *unopened* tiers before any
  acceptance attempt is standard Pandora's-Box semantics and safe; retrying a
  *previously-skipped* tier after a holdout rejection elsewhere is the same
  violation invariant #2 already forbids for same-tier resampling, and there
  is no state machine in `cascade.go` today that distinguishes the two cases.

**Validation path (offline, $0).** Checked directly against
`results/s55-n470.records.execution.json` (370 uncontaminated, oracle-sound
records, all three tiers present): non-nested crossings (a cheaper tier right
while a costlier one is wrong) occur at 2.2% (small↔mid), 1.1% (mid↔large),
0.5% (small↔large) — real, but roughly an order of magnitude rarer than the
nested direction at every pair (17.6%, 4.1%, 18.9%). This bounds the maximum
plausible benefit of *reordering* specifically; most available headroom is in
stopping/skipping, not permuting. A stopping-time rule (Phase 1) and even a
reordering rule that only conditions on already-recorded per-tier signals are
*both* offline-replayable against existing n=409/470 records, since `Profile`
already runs every tier on every problem regardless of policy.

**Recommendation.** Pursue Phase 1 (cheap, offline-validated, no invariant
risk, strict generalization of `preferRepair`). Do not build Phase 2 without
first exhausting Phase 1 — the measured non-nested rate (0.5–2.2%) is an
order of magnitude below the nested rate, and Phase 2's cost (re-keying the
certificate machinery) is the single riskiest refactor in the whole set.

**Open questions.** Is "adaptive ordering" meant as reordering or as
stop/continue (very different costs)? Should `Certificate.Thresholds` move
to identity-keyed regardless, ahead of any specific policy needing it? What
reservation-value estimator is defensible given errors are strongly
positively correlated across tiers?

## 2. Cross-family ensemble voting

**Mechanism.** Extend `config.Tier` to name several sub-models instead of
one, splitting `tier.Samples` across families (e.g. 1 sample each from
Maverick, Qwen, Haiku instead of 5 from one model). The architectural fit is
the best of all six: `cluster.Group`/`cluster.Score` (`internal/cluster/cluster.go:45-159`)
key purely on *executed outcome vectors*, never on source text or model
identity, so a mixed-family fan-out plugs in unmodified. This is a real
advantage over the existing self-consistency arm's `TextVote`
(`internal/cluster/text.go`), which clusters by normalized *source text* and
would fragment a cross-family vote for free — three families solving a
problem correctly but idiomatically differently would split a text vote even
though they agree behaviorally.

The complication is at repair time: `repairLoop` (`cascade.go:875-920`)
sends a failing candidate back to `tier.ModelID`, a single ID with no
notion of "the sub-model that actually wrote this one."

**Invariant fit.** #9 (Wilson lower bound) is respected cleanly — `Behaviour()`
and `wilsonLCB()` never see provenance. #1/#2 are respected by construction
(acceptance logic is unchanged), but a new attribution question appears at
repair: a mixed tier has no single "the tier's model" for `preferRepair`'s
cost formula, which assumes one `OutPerMTok`. #3 (test author ≠ code author)
sharpens the existing lesson about aliased model IDs: a multi-model tier
needs the *same* string-equality discipline applied per sub-model, or one
family in the mix could silently escape the `OracleContaminated` check.

**Validation path.** Partially answerable offline today, at low fidelity:
`results/qwen-coder-211.records.execution.json` (Qwen tier 0) and
`results/go-specialist-211-pinned-n64.execution.json` (Maverick tier 0) share
the same 64 problems and mid/large tiers, differing only in tier 0's model.
Intersecting on usable-in-both (n=47) and pairing each problem's tier-0
verdict simulates a synthetic 2-family vote without spending anything. Result:
**both agree-and-correct 38, both agree-and-wrong 3, disagree 6** (5
Maverick-only-wrong, 1 Qwen-only-wrong). Independence predicts ≈0.68
both-wrong events; 3 observed is 4.4× that (Fisher exact odds ratio ≈22.8,
p≈0.013) — on this slice, the *same* problems (`conc_fan_in_merge`,
`scale_chunk`, `scale_intersection`) fooled two differently-sourced cheap
models simultaneously. This is far too small and confounded to be a real
measurement (n=47, 2 families, no controlled joint sampling), but it is a
cheap existence proof that the core premise — different families'
mistakes are less correlated — is not obviously true here, and may be
weakly contradicted by the one comparison available.

A real test needs new live data: a config with tier 0 split across ≥2
families run on the same benchmark used elsewhere, priced with repair
enabled (per the "repair dominates tier-0 cost" lesson already in this
repo's history — a clean-sample probe was 1.9× optimistic).

**Recommendation.** Pursue with caveats, but re-run the correlated-failure
check across every other overlapping-problem-set record pair in `results/`
before spending anything live. If the correlation holds up broadly, the
diversity premise is in real trouble and the effort should redirect toward
understanding *why* certain problems fool everyone (a benchmark-property
question) rather than building a voting mechanism on a premise the data
argues against.

**Open questions.** Does the p≈0.013 signal survive a broader offline check?
How should `repairLoop`/`preferRepair` attribute cost for a mixed-family
candidate? Should a candidate carry a forensic-only model-provenance tag
(same pattern as `TierObs.DisagreementSource`)?

## 3. Problem decomposition / sub-task routing

**Mechanism.** The pipeline generates and verifies a candidate as one atomic
unit; the spec phase derives one public API and tests that call only that
API (`internal/prompt`), and `verify.Ladder` always builds/tests the whole
workspace. Two instantiations, which land very differently:

- **(1) Decompose-then-recombine at generation time** (the literal ask): a
  planning step invents sub-contracts, samples each independently, verifies
  each independently, then assembles. Two hard blockers: there is no oracle
  for an invented sub-contract (`Router.validateOracle`,
  `cascade.go:799-851`, validates the *whole-problem* contract against an
  execution-validated reference; no analog exists for an arbitrary internal
  decomposition), and if the assembled result fails acceptance, knowing
  *which* sub-part is at fault to selectively re-escalate requires a
  localization signal — and the only signal that revealed the failure is the
  hidden partition, which is exactly what invariant #1 forbids feeding back.
- **(2) Decompose only the repair step.** `repairLoop` already feeds a
  whole-file diagnostic from the *visible* partition back to the same model.
  A function-level version would localize a visible-test failure (via AST +
  `go test`'s file:line) to one function body and route a scoped patch to a
  pricier tier, instead of resampling the whole file. Verification stays the
  existing whole-file ladder unmodified — the function boundary only scopes
  the *prompt*, never the pass/fail semantics.

**Invariant fit.** (1) conflicts with #1 at reassembly if localization needs
hidden-test feedback, and introduces a brand-new unsound-oracle surface with
no validation path analogous to `calibrate -refs`. (2) respects #1 (still
only consumes the visible partition) and introduces no new oracle at all —
orthogonal to #4/#6.

A concrete check on this codebase's own reference corpus: the ~470 MultiPL-E
Go references average 1–2 top-level functions (max 6), so there is little to
decompose even where it would be safe to try.

**Recommendation.** Deprioritize the literal ask (1) — it is closer to the
unsolved multi-file/multi-unit orchestration problem `CLAUDE.md`'s Known
Gaps and the paper's §5.4 already flag as untackled than to a natural
extension of the current design. Pursue the narrow, invariant-compliant
version (2) instead: function-level repair localization, reusing
`verify/static.go`'s existing per-function AST facts.

**Open questions.** Can `go test` diagnostics plus AST reliably localize a
failure to one function across real control-flow shapes (shared helpers,
recursion) without ever consulting hidden tests? Does the reference corpus
contain enough genuinely multi-function problems to exercise this path at
all?

## 4. Specialized repair model, split from the generation model

**Mechanism.** Today repair is hard-wired to the tier's own model:
`repairLoop` calls `Generate` with `ModelID: tier.ModelID`
(`cascade.go:883`). There is no `Tier.RepairModel` field anywhere in
`internal/config/config.go` — this is not already configurable. A close
template exists, though: `Tier.PlannerModel` (`config.go:30-44`) already lets
a two-stage tier use a different model for planning than for coding, priced
separately via `PlannerCost()`, and checked against contamination
(`cascade.go:120-124`). `RepairModel` mirroring this pattern is mechanical.

The repair prompt itself (`prompt.RepairUser`, `internal/prompt/prompt.go:381-391`)
is already a narrow, "fix this diagnostic, do not restructure working code"
template, used regardless of which model authored the original attempt — so
the premise that repair is a narrower task than generation is already
reflected in the prompt design. What's genuinely untested is whether a
materially cheaper model can execute that narrow task reliably.

`preferRepair`'s marginal-gain priors (`gainRepair=0.45`, `gainEsc=0.35`,
`cascade.go:928-945`) are hardcoded, not measured, and implicitly assume the
repairer is exactly as capable as the tier's own coder — swapping in a
deliberately weaker model without re-deriving `gainRepair` risks
mis-prioritizing repair vs. escalation.

**Invariant fit.** #1 is untouched (repair still only sees visible-partition
diagnostics regardless of which model reads them). #3 is the sharp fit
question with a clear mechanical answer: `contaminated()` already treats
any model shaping the submitted code as a code author, so `RepairModel` must
be added to that same equality check or a collision with `TestModel` would
silently escape `OracleContaminated` — the same shape of gap this repo has
already had to fix twice (planner model, tier-level then cascade-level).

**Validation path.** The cost-accounting side is offline-checkable for $0
today: pull repair-specific cost (not just tier cost) out of the existing
n=409 records, per tier, and compare to total bill. Given tier 0 is already
measured at 0.8% of total spend in the closest existing analogue
(experiment 26), and four consecutive tier-0-model-swap experiments in this
repo's history were negative or near-null, this arithmetic should be done
*first* — before any code — because it bounds the maximum plausible saving.
The efficacy question ("would a cheap repair model have fixed what the
current repair fixed") cannot be answered by replay — no record exists of
what a different model would have produced on the same diagnostic — and
needs a live shadow-probe: re-send the same diagnostics to a candidate cheap
model, verify, log both outcomes without affecting any real run.

**Recommendation.** Deprioritize relative to other work, but the $0
arithmetic should be run regardless, since it is nearly free and directly
bounds whether the (small, well-precedented) code change is worth writing.
Every prior cost-lever experiment in this study's history points the same
direction: tier-0/repair-level interventions have a low ceiling because the
shared spec/oracle call dominates spend (74–91%).

**Open questions.** What fraction of *total* cost is attributable to repair
rounds specifically, at each tier? Does `RepairDepth:2` fire meaningfully at
mid/large tiers (not just tier 0), where a bad repair from a weak model is
riskiest? Should `RepairModel` be added to `Router.contaminated` and
`Config.Validate` before any live use?

## 5. Upfront difficulty classifier / predictive routing

**Mechanism.** Before `sequential`/`speculative` runs, a cheap classifier
`g(problem) → tier index k0` would replace the fixed `k := 0` start with a
predicted starting index, skipping tiers below it. This runs directly into
`Profile`'s core design property (`profile.go:14-18`, load-bearing for
invariant #8): every tier is sampled on every problem regardless of policy,
which is what makes offline threshold replay possible at all. A classifier
that physically skips sampling produces records with no outcome for the
skipped tiers — `Replay` cannot reconstruct what any threshold vector
touching that tier would have done, because the score doesn't exist for
those rows.

**Measured offline, $0, against `results/s55-fixed.records.execution.json`
(n=409, after excluding contaminated/oracle-unsound records):**

- First-correct-tier distribution: small=77%, mid=17%, large=3%, none=3% —
  massively skewed, so "always start at tier 0" is already a 77% baseline,
  hard for a cheap classifier to beat by much.
- A random forest over problem length, word count, `>>>`-example count, and
  keyword hits (concurrent/goroutine/sort/string — dead weight here, since
  MultiPL-E Go has zero concurrency problems) gets **5-fold CV accuracy
  0.582**, *worse* than the 0.770 majority baseline.
- Predicting "needs frontier or unsolved" specifically (the 6.1% class where
  skipping mid would help most) gets **AUC 0.427 — below chance.**
- Theoretical ceiling: a *perfect* classifier skipping tier 0 exactly where
  it fails saves only ~4.2% of realized cascade cost (tier 0 is already
  cheap); skipping to large on the ~6% needing it recovers more (~17.4%) but
  that is exactly the subclass the classifier cannot identify.

**Invariant fit.** Directly implicated: #7, #8, #9, #4, #6. The deepest
problem is the same shape as #8 (calibrate on the cache-bypass stream) even
though the mechanism differs — the classifier is deterministic/query-side,
not state-side, so it lacks the cache's non-stationarity, but it destroys the
same retrospective full-fan-out property invariant #8's whole machinery
depends on. #9 stays intact only if the classifier's confidence never
substitutes for or blends into the Wilson-bound acceptance score.

**Recommendation.** Deprioritize the tier-skipping variant as scoped — the
evidence is already negative on the one benchmark validated at scale, and the
one subpopulation where skipping ahead would matter most (needing frontier)
is exactly what a cheap classifier cannot identify. If pursued at all, the
safer framing is a hint fed into within-tier sampling (fan-out size, or
`carried` diagnostics) rather than a tier-skip — that stays fully orthogonal
to all nine invariants because it never changes which stage is sound or
which score is compared to a certified threshold.

**Open questions.** Would a stronger feature representation (embeddings, or
similarity to the arm-zero cache) do meaningfully better than length/keyword
features — this only rules out cheap surface features, not all cheap
classifiers. Is the 77/17/3/3 skew representative of a production stream or
an artifact of this benchmark's difficulty distribution?

## 6. Parallel tier racing (latency vs. cost)

**This one is already partly implemented.** `Router.Solve`
(`cascade.go:154`) picks `speculative` (line 196) over `sequential` (line
194) whenever `Deadline > 0` (`--deadline`, `cmd/go-cascade/main.go:111`).
The design intent is stated directly in the paper: "Dollars and latency rank
the verifier ladder in opposite directions... because parallelism buys
latency at the price of dollars" (`docs/verification-saturated-cascades.md:712-718`).

**Does firing tier 1 concurrently mean paying regardless of tier 0's
outcome?** Yes, mechanically: in `speculative` (`cascade.go:473-593`), every
fired tier's goroutine calls `sampleTier` and bills its cost before
acceptance even starts; there is no early-cancel-on-first-success — the
implementation drains the results channel until every tier finishes or hits
the deadline. It does not touch the certificate: each tier still gets
exactly one candidate and exactly one `acceptOne` call against the holdout,
unchanged from `sequential`. This is correctly framed as a separate
cost/latency frontier layered on an unchanged certificate, not a change to
the cost-vs-risk tradeoff itself.

**Does "take whichever passes first" violate invariant #2?** Not as
implemented, and precisely why: invariant #2 forbids a *second candidate from
the same tier* after that tier's holdout check has failed. `speculative`
never does this — each tier gets exactly one draw, one cluster, one
`acceptOne` call. Selection among *winners* is by cost order
(`slices.SortFunc`, line 544), not arrival order — structurally identical to
what `sequential` already does, just computed concurrently. The trap a naive
implementation could fall into: if a tier's *visible-partition* refutation
triggered a redraw from that same tier to "keep it in the race," that
reintroduces the invariant-#2 pattern relabeled as racing. The current code
does not do this.

**Where is the safe/risky line?** Safe: racing *within* a tier's already-
planned fan-out (parallelizing samples that were already going to be drawn).
Risky is not "racing across tiers before any holdout check" as a blanket
category — `speculative` does exactly that and stays sound, because
selection is a fixed, data-independent rule (cost order). The actual risky
version is selecting the winner by **wall-clock arrival instead of cost
order** — that doesn't break invariant #2 or certificate validity, but it
silently swaps "cheapest tier that suffices" for "whichever tier happened to
finish first under today's load," which is a cost-model bug (load
determining a routing outcome through an unrelated channel), not a soundness
bug.

**Recommendation.** Pursue with caveats — most of the pursuing is done. What's
missing is measurement (no quantification exists of the actual latency win
vs. dollar multiplier on a real benchmark) and one loose end:
`SkipRace: r.cfg.Deadline > 0` (`cascade.go:149`) unconditionally drops the
concurrency-detection rung under any deadline, coupling a latency choice to a
correctness-coverage decision with no apparent justification.

**Open questions.** Would a "hedge" (fire tier 1 only after a short delay,
rather than simultaneously) recover most of the latency win at a fraction of
the guaranteed extra spend? Should race-checking be its own independent
budget knob rather than riding along on the deadline setting?

---

## Cross-cutting synergies and tensions

Four independent reviews looked across all six findings at once — the check
individual design review is structurally unable to do, since each exploration
only saw its own approach.

### The two clearest synergies

- **Adaptive ordering (#1) and specialized repair (#4) are asking for the
  same missing number.** Both independently flag `preferRepair`'s
  `gainRepair=0.45` prior as unvalidated. A single live shadow-probe
  measuring real per-model fix rate against real diagnostics produces the
  calibrated marginal-gain-per-dollar quantity both need — #4 to replace the
  fixed prior with a model-dependent one, #1 to use the same generalized
  quantity as input to its own stopping rule.
- **Family voting (#2) and decomposition's narrow repair-localization
  cousin (#3.2) test complementary halves of the same hypothesis.**
  "Does a cheap specialist repair a whole file well?" and "does localizing
  the diagnostic help the *same* model repair faster?" are each individually
  underpowered; combined, they test the hypothesis that actually matters —
  does a cheap specialist repair *one localized function* well, the scope
  where its narrow-task premise has the best chance of holding.

### The clearest tension

- **Family voting (#2) and specialized repair (#4) want to change the exact
  same line of code in opposite directions.** #2's own invariant analysis
  says a mixed-family candidate "must be repaired by the model that wrote
  it" (per-candidate attribution). #4's whole point is decoupling repair from
  authorship entirely (one designated specialist, regardless of author).
  Combining them needs an explicit precedence rule neither proposal states,
  and doubles `preferRepair`'s degrees of freedom without doubling any
  evidence for it.

### The sharpest cross-cutting risk

**Every proposal that adds a new way to author or re-author a candidate —
family voting, specialized repair, decomposition's repair-localization —
independently adds exactly one new check to `Router.contaminated`, and no
single proposal enumerates the *other two's* authorship paths.** This
repo's own history shows this exact incremental-patch pattern has already
produced a silent contamination-check hole once (planner model vs. test
model, fixed at tier level, then again at cascade level, documented in
`CLAUDE.md`'s ARN-aliasing gotcha). Building two or three of these together
without one audit pass covering every authorship path at once is the
highest-risk moment in the entire set — not because any one change is
wrong, but because "the guarded branch simply stops running" leaves no
failing test.

### A secondary risk worth flagging even though it's currently hypothetical

Family voting's own offline check found that different cheap-model families'
*errors* co-occur roughly 4.4× more often than independence predicts
(Fisher p≈0.013, n=47) — the same problems fool multiple families at once.
If adaptive ordering (#1) is ever extended to treat cross-family *agreement*
as a signal for stopping early, agreement is least trustworthy exactly on
the problems where it would matter most — the same shape of failure the
self-consistency arm already hit (confidently wrong precisely where
execution reports nothing). Neither #1 nor #2 currently proposes this
combination; it is flagged so it isn't built later without re-deriving the
safety argument.

### Two approaches are largely redundant with each other

The difficulty classifier (#5) and family voting (#2) are both trying to
answer "will this need escalation" — #5 from static text features (measured
worse than the trivial baseline), #2 from real execution outcomes the
cascade generates anyway. Where both are in scope, #2's signal should be
preferred; #5 does not need to be built to get the benefit it was chasing.

## Recommended build order

1. **Free, first, zero invariant risk — RUN, 2026-08-08.**
   `results/design_space_offline_checks.py` broadens both checks against
   every available record file. Neither result favors building the
   corresponding approach:

   - **Family-voting's correlated-failure signal (#2) is not a small-n
     artifact — it replicates broadly.** Across 29 cross-family pairs with
     ≥15 overlapping problems (spanning maverick/qwen/haiku, every
     combination of record files sharing a problem set), **28/29 pairs
     show more co-occurring tier-0 failure than independence predicts**
     (12/29 significant at p<0.05, uncorrected), and the direction is
     consistent across all 13 maverick-vs-qwen pairs specifically. This
     weighs against the premise that different model families' mistakes
     are less correlated — the effect first seen at n=47 is not an outlier.
   - **Repair's dollar share of the total bill is small even under a
     generous proxy (#4).** Attributing *all* above-tier-median cost to
     repair (an overstatement — some spread is ordinary variance, not
     repair rounds) and summing across all three tiers on the n=488
     s55-fixed records: repair-attributable excess is 17.9% of tier-only
     spend. Tier-only spend is itself only 9–26% of the total bill (the
     remainder is the shared spec/oracle call, per `CLAUDE.md`). So
     repair's share of the *total* bill is **≈1.6–4.7%** — a low ceiling,
     consistent with every prior tier-0/repair-level cost lever in this
     study's history landing near-zero or negative net.

   Read together, these two results **do not kill either approach outright**
   (family voting stays architecturally the best fit of the six; specialized
   repair is still small and well-precedented to build), but they lower the
   expected value of building either right now relative to what the design
   review assumed going in. Revised recommendation: **build neither next**;
   re-rank both behind the sequencing below, and treat them as candidates to
   revisit if a later measurement (e.g. the shadow-probe adaptive-ordering
   and specialized-repair both independently want, see the synergy section)
   changes the picture.
2. **Drop now, on evidence already in hand.** Difficulty-classifier
   tier-skipping (#5) and adaptive-ordering Phase 2 / true reordering (#1).
   Both would touch the certificate's positional-indexing machinery — the
   riskiest surface in the codebase — for a benefit the existing data argues
   is small (#1: 0.5–2.2% non-nested crossings) or negative (#5: below-chance
   AUC on the class that matters).
3. **Build adaptive-ordering Phase 1** (stopping-time rule) first among the
   remaining candidates — promoted ahead of specialized repair and family
   voting now that step 1 has run: it is the one approach in the set whose
   gating check came back *unambiguously positive* (fully offline-replayable,
   zero live spend, no result contradicting it), where both #4 and #2 came
   back with real but small/unfavorable numbers. Fully offline-replayable,
   doesn't touch repair dispatch.
4. **Build specialized repair (#4) only if the shadow-probe below changes
   the arithmetic**, not as a standalone next step — step 1 found repair's
   ceiling on total spend is ≈1.6–4.7%, in the same range every prior
   tier-0/repair-level lever in this study has landed near-zero. If pursued
   anyway (e.g. because the shadow-probe measurement in the synergy section
   turns up a materially better fix rate than assumed), fold the
   `Router.contaminated`/`Config.Validate` extension into the same PR as a
   generalizable "list of author-shaping model IDs" check rather than one
   more hand-rolled clause — this establishes the pattern step 5 needs to
   extend rather than re-derive.
5. **Build family voting (#2) only after re-reading step 1's result**, not
   as a default next step — the correlated-failure signal broadened from
   one pair (p=0.013, n=47) to 28/29 pairs showing the same direction across
   every available cross-family comparison. The architectural fit is still
   the best of the six, but the core premise (diversity reduces correlated
   error) is now weighed against, not just unvalidated. If pursued, reuse
   step 4's generalized contamination pattern rather than re-deriving it,
   and treat the mechanism as "vote to detect disagreement and escalate,"
   not "vote and trust agreement" — agreement is exactly what this data
   says is least trustworthy on the hard subset.
6. **Build decomposition's narrow repair-localization cousin (#3.2)** last,
   after repair authorship has settled from steps 3 and 5 — it is the third
   change to touch the same `repairLoop` lines.
7. **Drop decomposition's literal instantiation (#3.1)** outright — no
   combination with anything else reduces its invariant #1 exposure or its
   missing-oracle problem.
8. **Racing (#6)** stands apart from this sequencing — it is already built.
   The remaining work (measuring the latency/dollar frontier, decoupling
   `SkipRace` from the deadline decision) has no dependency on 1–7 and can
   proceed independently at any point.
