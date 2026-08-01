# Kickoff prompt for the next session

Paste the block below to start the next session. It assumes memory is loaded
(MEMORY.md → gocascade-session-state.md has the full state).

---

We're continuing the go-cascade study. Before anything else:

1. **Check git state.** `main` should be at `021d7fe` (PR #33, paper reconciliation).
   33 PRs merged. Working tree clean, no PR open. If a PR *is* open, confirm CI
   green and I'll tell you whether to merge — don't merge unilaterally.
2. **Re-read `results/README.md`** (now SEVENTEEN experiments) and **`docs/verification-saturated-cascades.md` §5.6** (the live-evaluation reconciliation).
   `AWS_PROFILE=aws` for live Bedrock; `--provider=mock` is free.

## Where the study stands — a clean stopping point

Both original levers and all three NEXT-SESSION refinement threads are now run and
written up. The last session closed the study's open threads:

- **Experiment 16** (PR #31): cheap-planner two-stage (Haiku plans, Maverick codes).
  A cheaper planner mitigates but does NOT reverse the two-stage cost penalty
  (overhead 35×→8.7× vs #15's Opus; still 2.13× pricier at the certified thresholds).
  **Two planner points confirm: generalist-instructs-specialist is an accuracy lever,
  not a cost lever.**
- **Experiment 17** (PR #32): cost-win frequency — six 5:1:1 draws (three new, WITH
  `-compare`/β=0). **2 of 6 certify with a cost win, 3 of 6 certify-but-pricier, 1 no
  cert.** Exact rule: win iff 0 confident-wrong tier-0 in the clean set — and BOTH wins
  rode the oracle-soundness exclusion, not cheap-tier robustness. Sharper fragility
  claim than #14's "lucky draw."
- **Paper alignment** (PR #33): added dated **§5.6** reconciling the pre-live paper
  with the 17 experiments — "demonstrated at n≤64, NOT validated." Fixed the
  flatly-false pre-live sentences with pointers to §5.6; preserved the honesty record.

The paper and CLAUDE.md are now consistent with what was demonstrated vs. argued.

## Open threads (all are NEW scope — the refinement backlog is exhausted)

Nothing is half-done. The study is broad (17 experiments) and internally consistent.
Everything below is a fresh direction, not a loose end. Scope spend with me before
any live run.

1. **The real §5.5 validation experiment** (the one the paper says would validate,
   never yet run): n≥300 on a *standard* Go benchmark (HumanEval-Go / MBPP-Go /
   repo-level), all five arms, plus the two secondary tests §5.5(4) cache-warmth and
   §5.5(5) mutation-score-vs-*measured*-η_fa. This is large (real money, real
   engineering — benchmark ingestion + reference validation for 300+ problems) and is
   the single thing that would move the work from "demonstrated" to "validated." Scope
   carefully with me first.
2. **Plan-once-reuse-across-the-cascade** — the only untested two-stage variant with a
   shot at cost-positive: amortise ONE planner call over all tiers instead of charging
   it per-tier-0-query. This is a *structural code change* (the plan currently lives in
   `sampleTier`, regenerated per fan-out — see PR #28), not just config. Invariant-
   carrying package; read `docs/verification-saturated-cascades.md` before touching
   `internal/cascade`.
3. **§3.7 estimator test in isolation** (cheaper slice of #1's secondary): does
   mutation score track *measured* η_fa on problems where the reference permits
   ground-truth labelling? This is the §3.7 "unknown bias" question — the weakest link
   in the measurement→claim chain — and could be probed on the existing 64-problem set
   without a 300-problem benchmark.

Or: **declare the study done** and treat it as a finished artifact. It is at a
defensible stopping point.

Keep the discipline: branch-per-change + PR + green CI + I merge; surface confounds
rather than bury them (experiment 17 revised its own "2 of 6" into "2 of 6, and both
wins ride the soundness gate" — that's the bar); state demonstrated vs. argued; never
cite a mock number as a model measurement; launch long work with run_in_background,
react to notifications, don't sleep-poll; long calibrate runs checkpoint + `-resume`
on an external kill (proven on 5 real kills now); and 1Password SSH signing
intermittently fails on commit — ask me to unlock and retry.

---

## Quick reference (state as of 2026-08-01)

- Public repo: github.com/scttfrdmn/go-cascade. main `021d7fe`, 33 PRs merged, none open.
- **New this session:** `config.two-stage-haiku.json` (Haiku planner); records
  `two-stage-haiku-maverick-n64.{execution,judge}.json`,
  `go-specialist-511-draw-{d,e,f}.{execution,judge}.json`; write-ups
  `results/two-stage-haiku-2026-07-31.md`, `results/cost-win-frequency-2026-08-01.md`;
  paper §5.6 reconciliation.
- **Configs:** `config.go-specialist-{211,321,511}.json`, `config.two-stage.json`
  (Opus plans), `config.two-stage-haiku.json` (Haiku plans). test_model = sonnet-4-6,
  MUST differ from every tier AND every planner (invariant #3).
- **Analysis scripts (offline, free):** `results/headroom_theorem.py`,
  `results/analyze_tension.py`, `results/analyze_draws.py` (feed it the six 5:1:1
  draws a–f; a/b/c are #13/#14, d/e/f are #17 with `-compare`).
- **Live spend this project so far:** ~$95–105 (prior ~$80–85 + this session's ~$16–20:
  cheap-planner ~$4-6, three 5:1:1 `-compare` draws ~$12-15).
- Bedrock models ACTIVE (us-west-2, as of this session): maverick-17b, haiku-4-5,
  sonnet-4-5, sonnet-4-6, opus-4-5 (+ many newer opus/sonnet/fable-5 profiles now
  listed — keep configs pinned to the models each experiment used for cross-run
  cost comparability). Re-run `go-cascade models` to confirm before any live run.
