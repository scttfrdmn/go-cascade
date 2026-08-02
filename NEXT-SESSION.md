# Kickoff prompt for the next session

Paste the block below to start the next session. It assumes memory is loaded
(MEMORY.md → gocascade-session-state.md has the full state).

---

We're continuing the go-cascade study. Before anything else:

1. **Check git state.** `main` should be at `c5711ec` (PR #40, experiment 19).
   40 PRs merged. Working tree clean, no PR open. If a PR *is* open, confirm CI
   green and I'll tell you whether to merge — don't merge unilaterally.
2. **Re-read `results/README.md`** (now NINETEEN experiments) and
   **`docs/verification-saturated-cascades.md` §5.6** (the live-evaluation
   reconciliation, now including the §5.5(5) result).
   `AWS_PROFILE=aws` for live Bedrock; `--provider=mock` is free.

## Where the study stands

**Both original levers, all three refinement threads, and both of last session's
new code threads are now run and written up.** The last session closed the two
substantive threads it opened and fixed a real robustness bug found while doing so:

- **Experiment 18** (PR #35 code, #38 results): **plan-once-reuse-across-the-cascade**
  — one planner call per query threaded into *every* tier (`Config.PlannerModel`),
  vs the per-tier planner of #15/#16. **Negative, and it explains why.** Tier-0
  accuracy 0.885 ≈ no-plan 0.88 and *below* both per-tier arms; ~2× pricier at the
  certified α=0.15. Mechanism: amortisation requires the cheap tier to *accept*, but
  at 2:1:1 the 2-sample Wilson ceiling (0.425) sits below every certifiable
  threshold, so 0/52 clean tier-0 answers can clear it, every query escalates, and
  the one plan has nothing to amortise across. **Three planner points now close the
  two-stage question: no plan-placement variant reverses "accuracy lever, not cost
  lever."**
- **Experiment 19** (PR #37 code, #40 results): the **§3.7 estimator test**, §5.5(5).
  Non-circular by construction — M against the *generated* suite, correctness against
  each problem's *human-authored* canonical suite. **η_fa = 0/144** (95% bound 0.021)
  against a pooled 1−M of **0.0996** that predicted ~11 events. So §3.7's "the
  direction of the net bias is not known to us" now has an answer *on this
  benchmark*: **conservative** — the safe direction. Two live caveats, both written
  up: whether M *ranks* candidates by η_fa is **unresolved** (0 events in both M
  buckets; the 12 lowest-M rows are all canonically correct, so low M was an
  artifact), and the generated oracle's observed errors are **all over-rejections**
  (11 confirmed, 7.1% of labeled rows, vs 0 false acceptances) — a
  **sound-but-stricter-than-canonical** hazard class neither §3 nor the `-refs` gate
  models, which costs escalations rather than risk and so is invisible to the
  certificate.
- **PR #39: atomic checkpoint writes.** `writeJSONFile` used `os.WriteFile`, which
  truncates the target to zero *before* writing — an external SIGTERM in that window
  destroyed all progress and defeated `-resume`. It cost 31 problems of live spend
  before being found. Now temp + fsync + rename (+ chmod 0644; `os.CreateTemp` is
  0600). **Proven live the same day**: the estimator run was killed at 30/64 and
  31/64 and resumed with zero loss.

The paper, `results/README.md`, and CLAUDE.md are all consistent with what has been
demonstrated vs. argued.

## The single open gap

Everything else is exhausted. **One thread remains, and it is the only one that
would change the study's status:**

1. **The real §5.5 validation experiment.** n ≥ 300 on a *standard* Go benchmark
   (HumanEval-Go / MBPP-Go / repo-level), all five arms, plus §5.5(4) cache-warmth.
   This is the single thing that moves the work from "**demonstrated** at n≤64" to
   "**validated**." It is large: real money *and* real engineering (benchmark
   ingestion + execution-validated references for 300+ problems, since the `-refs`
   oracle-soundness gate and the canonical-suite oracle both depend on them).
   **I said "not yet" to this twice — ask before assuming it's live, and scope the
   spend with me first.**

   Two n=64 findings specifically await it: whether **M ranks** candidates by η_fa
   (experiment 19 had no events to rank), and whether the **cost win** is more than
   a 2-in-6 coincidence (experiment 17).

Or: **declare the study done** and treat it as a finished artifact. Nineteen
experiments, both levers mapped, both secondary tests one-of-two run, every claim
labelled demonstrated-vs-argued. It is at a defensible stopping point.

Keep the discipline: branch-per-change + PR + green CI + I merge; surface confounds
rather than bury them (experiment 19 reported its own null result on the
discriminative question rather than leading with the headline — that's the bar);
state demonstrated vs. argued; never cite a mock number as a model measurement;
long runs checkpoint + `-resume`.

**Ops, learned the hard way last session:** long live runs are SIGKILLed by the
harness's background-task reaper (6+ times now), *not* by Bedrock. Use both defenses:
`-resume` with the atomic checkpoints, **and launch detached** in its own process
session — `setsid` does not exist on macOS, so double-fork via python (`os.fork` →
`os.setsid` → `os.fork` → `os.execve`), then poll with `pgrep`/`tail`. Don't
sleep-poll a foreground job. 1Password SSH signing intermittently *hangs* the commit
for the full timeout — ask me to unlock and retry.

---

## Quick reference (state as of 2026-08-01, end of third session)

- Public repo: github.com/scttfrdmn/go-cascade. main `c5711ec`, 40 PRs merged, none open.
- **New this session:** `go-cascade estimator` subcommand + `Config.PlannerModel`
  (cascade-level planner) + atomic `writeJSONFile`; `config.plan-once.json`; records
  `plan-once-n64.{execution,judge}.json`, `estimator-n64.json`; write-ups
  `results/plan-once-reuse-2026-08-01.md`,
  `results/estimator-test-{design,n64}-2026-08-01.md`; paper §3.7 + §5.5(5) + §5.6
  updates.
- **Configs:** `config.go-specialist-{211,321,511}.json`, `config.two-stage.json`
  (Opus plans, per-tier), `config.two-stage-haiku.json` (Haiku plans, per-tier),
  `config.plan-once.json` (Haiku plans, cascade-level). test_model = sonnet-4-6,
  MUST differ from every tier AND every planner (invariant #3).
- **Analysis scripts (offline, free):** `results/headroom_theorem.py`,
  `results/analyze_tension.py`, `results/analyze_draws.py` (feed it the six 5:1:1
  draws a–f; a/b/c are #13/#14, d/e/f are #17 with `-compare`).
- **Live spend so far:** ~$108–126 (prior ~$95–105 + experiment 18 ~$5–7 +
  experiment 19 ~$8–14, the latter *estimated not measured* — the `estimator`
  subcommand records no cost field — and including ~$4–7 lost to the pre-atomic-write
  killed run).
- Bedrock models ACTIVE (us-west-2): maverick-17b, haiku-4-5, sonnet-4-5, sonnet-4-6,
  opus-4-5 (+ newer opus/sonnet/fable-5 profiles — keep configs pinned to the models
  each experiment used, for cross-run cost comparability). Re-run `go-cascade models`
  to confirm before any live run.
