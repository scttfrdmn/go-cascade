# Pilot #2 — paired oracle comparison — 2026-07-25

Second live run, fixing the confounder in pilot #1. Both oracles rule on a
**single shared candidate stream** (`ProfilePaired`): one spec, one set of
samples per tier, the same representative submitted to both the held-out tests
(execution) and the reading-only judge. Any arm-to-arm difference is therefore
attributable to the oracle, not to independent sampling. 28 problems, α=0.10,
δ=0.10, zero skips. Records saved to `results/pilot2-paired.{execution,judge}.json`
(replayable offline at any α).

## Result

| arm       | valid | cert-α | empirical risk | realized risk | verdict           |
|-----------|-------|--------|----------------|---------------|-------------------|
| execution | false | 0.100  | 0.1071         | 0.1071        | could not certify |
| judge     | false | 0.100  | 0.1429         | 0.1071        | could not certify |

Both arms saw identical candidates, so realized risk (ground truth) is the same
0.1071 for both — the programs are the same; only the oracle's opinion differs.

## The oracle divergences (this is the point)

Comparing the judge's verdict to execution ground truth on the same programs,
across all tiers:

**Judge false acceptances — η_fa (passed a program the tests refute): 3, all on
`conc_parallel_map`, at every tier.**
A data race is invisible to a reader; execution catches it under `-race`. The
judge passed the racy solution consistently at small/mid/large, which is exactly
§3.1's claim — the judge certifies against its own verdict and *cannot see* this
defect class. This is why the judge's empirical risk (0.1429, what it would
certify) is **higher** than its realized risk here: on this benchmark the judge's
conservatism happens to reject more than it wrongly passes, but the passes it
does make include defects it is structurally blind to.

**Judge false rejections — β>0 (failed a correct program): 8 tier-instances
across 6 problems** (str_anagram, slice_most_frequent, str_valid_ipv4,
conc_parallel_filter, conc_fan_in_merge, conc_first_success, conc_bounded_pipeline).
The mock never modelled this: a real judge is not merely permissive, it is also
wrongly conservative. β≠0 for the judge; β=0 for execution by construction.

## What this establishes

- **Execution soundness held again, live and paired:** realized == empirical
  (0.1071), β=0. 3 of 28 accepted-at-tier-0 programs were genuinely wrong.
- **The executable oracle and the LLM judge disagree with ground truth in
  opposite ways.** Execution is sound but silent on things tests miss; the judge
  is wrong in *both* directions (η_fa>0 AND β>0) on the identical candidates.
  This is the qualitative §3.1 picture, now isolated from sampling noise.

## What this still does NOT establish

- **The headline "judge certifies lower α while being wrong" did not cleanly
  reproduce** in the α-comparison sense: here the judge's *empirical* risk was
  higher than execution's, because its false rejections outnumbered its false
  acceptances on this easy-leaning benchmark. §3.1's danger (a judge certifying
  BELOW its true risk) needs a benchmark where η_fa dominates β — i.e. more
  subtle, reader-invisible defects (races, aliasing, spec misreadings) and fewer
  problems a careful reader nails. That is a benchmark-design task.
- **n=28 still cannot certify at α=0.10** (empirical risk ~0.11–0.14 with these
  errors); the sample-size ceiling stands.
- **Judge β is inflated by a strict prompt.** The judge prompt says "when in
  doubt, FAIL", which deliberately trades false acceptances for false rejections.
  A different operating point would move the η_fa/β balance; the prompt is a knob,
  not a fixed property of the model.

## Follow-ups

1. **A harder benchmark weighted toward reader-invisible defects**, to test
   whether η_fa can be driven above β and reproduce the §3.1 danger directly.
2. **Sweep the judge prompt's strictness** to trace its η_fa/β operating curve.
3. **Larger n** toward the §5.5 target so certification becomes possible.

Live spend this session: roughly $10–13 including both pilots and diagnosis.
