# Judge strictness sweep — 2026-07-25

Live `--judge-sweep`: paired `--compare` run once per judge strictness level
(strict / balanced / permissive) on a 6-problem defect-prone subset (the pilot's
race problem, two more concurrency problems, and three subtle-logic problems).
α=0.10, δ=0.10, records saved to `results/sweep.<level>.{execution,judge}.json`.
18 paired problem-runs, zero skips.

Hypothesis: loosening the judge's PASS/FAIL tie-break should trade false
rejections (β) for false acceptances (η_fa), and if so, elicit the §3.1
dangerous mode (a judge certifying below its true risk).

## Result

| strictness | exec-risk | judge-emp | judge-real | η_fa | β |
|------------|-----------|-----------|------------|------|---|
| strict     | 0.1667    | 0.0000    | 0.1667     | 3    | 2 |
| balanced   | 0.0000    | 0.0000    | 0.0000     | 0    | 4 |
| permissive | 0.1667    | 0.1667    | 0.1667     | 3    | 2 |

## Two real findings

**1. The §3.1 dangerous mode IS reachable — and appeared even at "strict".**
In the strict and permissive runs the judge false-accepted `seq_longest_run` at
all three tiers: a subtly wrong solution (the `>=`-for-`>` off-by-one that passes
the visible tests and fails the hidden partition) that the judge read as clearly
correct. Execution refuted it (exec-risk 0.167); the judge passed it. That is a
judge certifying below true risk — the danger the earlier easy/hard runs never
showed, now observed on a non-concurrency problem. Note the permissive row is the
cleanest instance: judge-empirical (0.167) matches judge-realized (0.167) only
because it also accepted the wrong program, i.e. the judge's own risk estimate
and the truth coincide *by luck*, not by soundness.

**2. The sweep could NOT attribute this to strictness — a design limitation.**
Strict and permissive produced *identical* per-tier scores and truth for
`seq_longest_run` (0.65/0.42/0.27, all wrong), i.e. they sampled equivalent
candidates and **both** passed them. The tie-break instruction did not change the
verdict, because the judge was not "in doubt" on this candidate — it was
confidently wrong. Strictness only moves verdicts the judge is *uncertain* about;
a confidently-misread defect is immune to the knob. And because each level
re-samples candidates independently (ProfilePaired regenerates per call), the
levels faced different programs overall, so the η_fa/β differences between rows
are dominated by sampling, not by strictness. The balanced row's all-different
divergences are the tell.

## Honest conclusion

- **Dangerous mode: demonstrated.** A reading-only judge does sometimes certify a
  wrong program (η_fa>0), including subtle non-concurrency defects, so §3.1's
  concern is real and observed — the executable oracle caught every case the
  judge missed.
- **"Reachable by loosening the prompt": not demonstrated.** The knob moves
  only doubt-boundary cases; the false acceptances here were confident misreads,
  unaffected by strictness. A controlled test would need the SAME candidate
  stream judged at every strictness level (judge each cached candidate three
  times), not an independent re-sample per level. That is the clean follow-up.

## Follow-up

Make `--judge-sweep` cache the sampled candidates once and replay them through
each strictness level, so strictness is an A/B on identical programs. Only then
can the η_fa/β *curve* be attributed to the knob rather than to sampling.

Live spend this session: roughly $17–20 across both pilots, the hard tier, and
the sweep.
