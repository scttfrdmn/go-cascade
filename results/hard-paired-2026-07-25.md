# Hard-tier paired comparison — 2026-07-25

Live paired `--compare` on the hard benchmark tier (`examples/bench/hard/`), 12
problems built to concentrate on reader-invisible defect classes (overflow,
aliasing, Unicode, concurrency). α=0.10, δ=0.10, shared candidate stream, zero
skips. Records in `results/hard-paired.{execution,judge}.json`.

## Hypothesis under test

The pilot (#8) could not reproduce the §3.1 *danger* — a judge certifying below
its true risk — because the judge's false rejections (β) outnumbered its false
acceptances (η_fa). This tier was built to invert that: subtle, hard-to-read-
correctly problems where a plausible-but-wrong candidate should slip past a
reading-only judge, driving η_fa up.

## Result

| arm       | valid | cert-α | empirical risk | realized risk |
|-----------|-------|--------|----------------|---------------|
| execution | false | 0.100  | 0.0000         | 0.0000        |
| judge     | false | 0.100  | 0.1667         | 0.0000        |

Divergences (judge verdict vs execution truth, all tiers):

- **Judge false-acceptances (η_fa): 0**
- **Judge false-rejections (β): 6** — `hard_num_pow_mod` (all 3 tiers),
  `hard_num_mean_overflow` (large), `hard_conc_ordered_fanout` (small, mid).

## What actually happened — the hypothesis was wrong, and that is the finding

The §3.1 danger did **not** reproduce; the opposite did. On these problems the
models actually produced **correct** solutions (execution risk 0.0000 — they
handled overflow with `math/big`, wrote race-free concurrency, etc.). The judge,
reading `math/big` usage and concurrent code it could not fully trace, **grew
suspicious and rejected correct programs**. Making problems harder to verify by
reading did not make the judge accept more wrong code; it made the judge reject
more right code. β went up, η_fa stayed at zero.

So across both live tiers the picture is consistent and, if anything, stronger
than the paper's framing in one respect and weaker in another:

- **Stronger:** a reading-only judge is not merely noisy — on exactly the subtle
  code where correctness is hardest to establish, it is *uselessly conservative*,
  rejecting correct solutions the executable oracle certifies at 0 risk. A
  cascade gated on this judge would escalate or fail these, paying more for worse
  outcomes.
- **Weaker / not shown:** we did **not** observe the dangerous mode (η_fa > 0
  leading to certification below true risk) on either tier. With Claude
  Sonnet 4.6 as judge and a "when in doubt, FAIL" prompt, false acceptances were
  rare-to-absent. The §3.1 danger is real in principle but was not exhibited
  here; a permissive judge prompt, a weaker judge model, or defects that read as
  *confidently correct* (rather than *hard to read*) would be needed to elicit it.

## Caveats

- n=12; these are point estimates with wide intervals. Neither arm certifies at
  α=0.10.
- The judge's β is sensitive to the prompt's strictness ("when in doubt, FAIL").
  A permissive prompt would trade these false rejections for some false
  acceptances and is the obvious next sweep.
- "Hard to read" and "reads as confidently correct" are different axes. This tier
  hit the first; eliciting η_fa needs the second (e.g. a subtly wrong solution
  that looks clean, like an off-by-one in a tidy loop, rather than an
  intimidating `math/big` block).

## Combined read of both live tiers

| tier | n | exec risk | judge η_fa | judge β |
|------|---|-----------|------------|---------|
| pilot (easy) | 28 | 0.107 | 3 (all conc_parallel_map, a race) | 8 |
| hard | 12 | 0.000 | 0 | 6 |

The one clear η_fa signal in either run was a **data race** the judge could not
see (pilot). Overflow and `math/big` correctness, by contrast, the judge tended
to *over*-reject. Reader-invisibility helps the executable oracle either way: it
catches the race the judge misses, and it certifies the correct-but-hard-to-read
code the judge wrongly fails.

Live spend this session: roughly $13–16 across both pilots, the hard tier, and
diagnosis.
