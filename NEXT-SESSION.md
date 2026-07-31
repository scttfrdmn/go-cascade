# Kickoff prompt for the next session

Paste the block below to start the next session. It assumes memory is loaded
(MEMORY.md → gocascade-session-state.md has the full state).

---

We're continuing the go-cascade live evaluation. Before anything else:

1. **Check git state.** `main` should be at `63a9473` (PR #29, two-stage arm).
   29 PRs merged. Working tree clean, no PR open. If a PR *is* open, confirm CI
   green and I'll tell you whether to merge — don't merge unilaterally.
2. **Re-read `results/README.md`** (now FIFTEEN experiments). The two most recent
   arcs: the deployable-α confound (#12→#14) and the two-stage arm (#15).
   `AWS_PROFILE=aws` for live Bedrock; `--provider=mock` is free.

## Where the study stands (both original levers now RUN)

The last session closed the two levers the README flagged as open:

- **Deployable α=0.05 is reachable** (experiment 12, first in the study) once the
  `-refs -pin-api` gate removes spec-model test noise — genuine model error was
  0/52. **But** the α=0.05 *cost win* is **fragile, not guaranteed** (experiments
  13–14): a higher cheap-tier fan-out (5:1:1) buys threshold headroom against
  *flaky* cheap-tier errors and *none* against *confident* ones (proved:
  `results/headroom_theorem.py`), and across 3 fresh 5:1:1 draws only 1 certified
  with a cost win — the others hit a confident cheap-tier error (no win) and a
  top-tier miss (no cert). At n≈53 one error anywhere moves the certificate;
  binding constraint is **model accuracy at small n**.
- **Generalist-instructs-specialist is an accuracy lever, not a cost lever**
  (experiment 15). Opus-plans/Maverick-codes nudged tier-0 accuracy 0.88→0.92
  (noise-band) but the Opus planner on every tier-0 query made the cascade **3.1×
  pricier than always-frontier**. The cheap-bottom-tier win (#11) beats it.

## Open refinement threads (pick one, or decide it's write-up time)

All three are refinements, not new territory — the study is broad (15 experiments)
and at a plausible stopping/write-up point. Scope spend with me before any live run.

1. **Cheap-planner two-stage variant** — build `config.two-stage-haiku.json`
   (Haiku/Nova planner, NOT Opus/Sonnet — the only pairing with a shot at net cost
   benefit) and run it (~$4-6). Does a cheap plan salvage the two-stage economics?
   The two-stage tier code is merged (PR #28); this is config + a live run only.
2. **Cost-win frequency** — 2-3 more 5:1:1 draws WITH `-compare` (for β=0 ground
   truth; the #14 draws b/c lacked it) to turn "1 of 3 certified with a cost win"
   into an actual frequency estimate (~$12-18).
3. **Write-up / paper alignment** — the live results now cover §5.5 pieces the
   paper argued; consider reconciling `docs/verification-saturated-cascades.md`
   with what was demonstrated vs. what remains argued.

Keep the discipline: branch-per-change + PR + green CI + I merge; surface confounds
rather than bury them (last session revised its own #13 headline when repeat draws
showed the cost win didn't replicate — that's the bar); state demonstrated vs.
argued; never cite a mock number as a model measurement; launch long work with
run_in_background, react to notifications, don't sleep-poll; long calibrate runs
checkpoint + `-resume` on an external kill (proven on 3 real kills last session);
and 1Password SSH signing intermittently fails on commit — ask me to unlock and
retry.

---

## Quick reference (state as of 2026-07-31)

- Public repo: github.com/scttfrdmn/go-cascade. main `63a9473`, 29 PRs merged, none open.
- **Two-stage tier is now a shipped feature** (PR #28): `config.Tier.PlannerModel`
  (+ `PlannerIn/OutPerMTok`); empty = single-stage. Planner call in `sampleTier`
  before the fan-out; `prompt.PlanSystem/PlanUser` + `CodeUserFromPlan`;
  `model.PurposePlan`. Invariant #3 enforced at `config.Validate` AND both
  `OracleContaminated` accept sites. `examples/bench/config.two-stage.json` =
  Opus/Maverick starting config.
- **New analysis scripts** (offline, free, re-derivable): `results/headroom_theorem.py`
  (fan-out flaky-vs-confident dichotomy), `results/analyze_tension.py` (why a draw's
  τ0 collapses), `results/analyze_draws.py` (aggregate draws: flaky/confident/refuted
  split + acceptance-risk events).
- **Configs:** `config.go-specialist-{211,321}.json` (Maverick tier0, 2:1:1/3:2:1),
  `config.go-specialist-511.json` (5:1:1), `config.two-stage.json` (Opus plans).
  test_model = sonnet-4-6, MUST differ from every tier AND every planner (#3).
- **Live spend this project so far:** ~$80-85 (prior ~$65-69 + this session's ~$15:
  n=64 pinned ~$2.2, 2 draws ~$2, two-stage ~$4.6, + spec/pin/planner overhead).
- Bedrock models ACTIVE (us-west-2): maverick-17b, opus-4-5, sonnet-4-5, sonnet-4-6.
