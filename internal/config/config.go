// Package config holds the tier definitions, cost model and risk knobs that
// parameterise the cascade.
//
// The single free knob is lambda, the shadow price of correctness in dollars
// per unit error probability. Callers normally set alpha (a risk target) or a
// budget and let the calibrator recover lambda; see internal/calibrate.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/scttfrdmn/go-cascade/internal/cluster"
)

// Tier is one rung of the model cascade. Costs are in USD per million tokens.
type Tier struct {
	Name        string  `json:"name"`
	ModelID     string  `json:"model_id"`
	InPerMTok   float64 `json:"input_usd_per_mtok"`
	OutPerMTok  float64 `json:"output_usd_per_mtok"`
	Samples     int     `json:"samples"`      // n candidates drawn for behavioural clustering
	RepairDepth int     `json:"repair_depth"` // max repair rounds before escalating
	Temperature float32 `json:"temperature"`

	// PlannerModel, when set, makes this a two-stage tier: a single call to the
	// planner model rewrites the problem into an implementation plan, and every
	// coder sample (ModelID) is generated *from that plan* instead of the raw
	// problem. Empty means a plain single-stage tier (the default). The planner
	// is a prompt-construction step only — it never touches the oracle or the
	// verify ladder, so soundness (invariant #4) is unaffected — but because it
	// materially shapes the submitted code it is bound by invariant #3: it must
	// differ from TestModel or the record is oracle-contaminated (see Validate
	// and Router acceptance).
	PlannerModel string `json:"planner_model,omitempty"`
	// PlannerInPerMTok/OutPerMTok price the planner call when it is a different
	// model than the coder. Zero falls back to the coder's own prices so a
	// same-model two-stage tier needs no extra config.
	PlannerInPerMTok  float64 `json:"planner_input_usd_per_mtok,omitempty"`
	PlannerOutPerMTok float64 `json:"planner_output_usd_per_mtok,omitempty"`
}

// Cost returns the USD cost of a single coder call with the given token counts.
func (t Tier) Cost(inTok, outTok int) float64 {
	return float64(inTok)/1e6*t.InPerMTok + float64(outTok)/1e6*t.OutPerMTok
}

// TwoStage reports whether this tier runs a planner call before the coder.
func (t Tier) TwoStage() bool { return t.PlannerModel != "" }

// PlannerCost prices a single planner call. It uses the planner's own per-MTok
// prices when set, otherwise the coder's — so a two-stage tier that reuses the
// same model needs no separate pricing, while a distinct planner is priced
// correctly rather than at the (usually cheaper) coder's rate.
func (t Tier) PlannerCost(inTok, outTok int) float64 {
	in, out := t.PlannerInPerMTok, t.PlannerOutPerMTok
	if in == 0 {
		in = t.InPerMTok
	}
	if out == 0 {
		out = t.OutPerMTok
	}
	return float64(inTok)/1e6*in + float64(outTok)/1e6*out
}

// Config is the whole knob surface.
type Config struct {
	Region string `json:"region"`

	// Tiers are ordered cheapest first. Tier 0 of the *cascade* is the cache,
	// which is not a model and so does not appear here.
	Tiers []Tier `json:"tiers"`

	// TestModel writes the oracle. It must differ from the tier that writes the
	// code or the oracle is contaminated; the cascade records this in the trace.
	TestModel   string  `json:"test_model"`
	TestInCost  float64 `json:"test_input_usd_per_mtok"`
	TestOutCost float64 `json:"test_output_usd_per_mtok"`

	// ComputeUSDPerCoreSecond prices verifier time. Default is roughly a
	// c7g.large on-demand rate divided by two vCPU.
	ComputeUSDPerCoreSecond float64 `json:"compute_usd_per_core_second"`

	// Alpha is the target risk (probability of returning incorrect code).
	// Budget, if non-zero, is a hard USD ceiling per query. Setting both is an
	// error: they are dual descriptions of the same constraint.
	Alpha  float64 `json:"alpha"`
	Budget float64 `json:"budget_usd"`

	// PlannerModel, when set, makes the WHOLE cascade two-stage: a single planner
	// call per query rewrites the problem into an implementation plan that EVERY
	// tier's coder works from. It differs from a *tier's* PlannerModel, which
	// draws and charges a plan per tier and lifts only that one tier: here one
	// plan is drawn at cascade entry and threaded into all tiers, so its cost
	// amortises across escalation instead of being sunk at whichever tier drew it.
	// Empty means no cascade-level planner. Like a tier planner it is a
	// prompt-construction step that never touches the oracle (soundness/invariant
	// #4 unaffected) but shapes the submitted code, so invariant #3 binds: it must
	// differ from TestModel (Validate rejects equality). Mixing it with any tier's
	// own PlannerModel is rejected — the two plan sources would double-charge and
	// muddy cost attribution.
	PlannerModel string `json:"planner_model,omitempty"`
	// PlannerInPerMTok/OutPerMTok price the cascade planner call. Zero falls back
	// to tier 0's coder prices (the tier the plan cost is attributed to).
	PlannerInPerMTok  float64 `json:"planner_input_usd_per_mtok,omitempty"`
	PlannerOutPerMTok float64 `json:"planner_output_usd_per_mtok,omitempty"`

	// Deadline flips the topology from sequential cascade to speculative
	// parallel. Zero means sequential.
	Deadline time.Duration `json:"deadline"`

	// Deterministic objectives. These are measurements, not predictions, so
	// they cost nothing against the risk budget. Zero disables.
	MaxComplexity int `json:"max_cyclomatic_complexity"`
	MaxAllocsOp   int `json:"max_allocs_per_op"`

	// Verifier ladder controls.
	TestTimeout time.Duration `json:"test_timeout"`
	RaceCount   int           `json:"race_count"`  // -count for the race stage
	Mutants     int           `json:"mutants"`     // 0 disables oracle-gap estimation
	StdlibOnly  bool          `json:"stdlib_only"` // hard filter on imports

	// Cache.
	CacheDir     string  `json:"cache_dir"`
	CacheTopK    int     `json:"cache_top_k"`
	CacheMinSim  float64 `json:"cache_min_similarity"`
	CacheAdmitAt float64 `json:"cache_admit_score"` // stricter than alpha
	ShadowRate   float64 `json:"shadow_rate"`       // fraction routed past the cache

	// ExecWrapper, if set, prefixes every `go test` invocation, e.g.
	// "bwrap --unshare-net --ro-bind / /". Model-authored code is executed.
	ExecWrapper []string `json:"exec_wrapper"`

	// JudgeStrictness sets the judge oracle's PASS/FAIL boundary on uncertainty
	// ("strict", "balanced", "permissive"). It only affects the judge-comparison
	// arm and is the knob that traces the judge's false-acceptance/false-rejection
	// operating curve. Empty means strict.
	JudgeStrictness string `json:"judge_strictness"`

	ThresholdsPath string `json:"thresholds_path"`
}

// Default returns a working configuration.
//
// Model IDs move faster than this file does. These are Bedrock inference
// profile IDs, not bare model IDs (on-demand throughput requires a profile).
// Verify against your account with `go-cascade models`, which calls
// ListInferenceProfiles, and override with --tier-model or a config file.
func Default() *Config {
	c := defaults()
	c.CacheAdmitAt = c.DefaultAdmitScore()
	return c
}

func defaults() *Config {
	return &Config{
		Region: "us-west-2",
		Tiers: []Tier{
			{
				Name: "small", ModelID: "us.anthropic.claude-haiku-4-5-20251001-v1:0",
				InPerMTok: 1.00, OutPerMTok: 5.00,
				Samples: 5, RepairDepth: 2, Temperature: 0.8,
			},
			{
				Name: "mid", ModelID: "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
				InPerMTok: 3.00, OutPerMTok: 15.00,
				Samples: 2, RepairDepth: 2, Temperature: 0.4,
			},
			{
				Name: "large", ModelID: "us.anthropic.claude-opus-4-5-20251101-v1:0",
				InPerMTok: 5.00, OutPerMTok: 25.00,
				Samples: 1, RepairDepth: 2, Temperature: 0.2,
			},
		},
		TestModel:   "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		TestInCost:  3.00,
		TestOutCost: 15.00,

		ComputeUSDPerCoreSecond: 0.0000133,

		Alpha: 0.05,

		TestTimeout: 30 * time.Second,
		RaceCount:   3,
		Mutants:     24,
		StdlibOnly:  true,

		CacheDir:    defaultCacheDir(),
		CacheTopK:   5,
		CacheMinSim: 0.35,
		// Admission has a ceiling, and the old default of 0.90 was above it. The
		// routing score is a Wilson lower bound, not raw cluster mass (invariant #9),
		// so a *unanimous* tier of n samples reports 0.2698 at n=1 and 0.6488 at n=5,
		// never 1.0 — which silently made PutSolution unreachable and the arm-zero
		// solutions layer dead in every experiment in results/. Nothing failed: an
		// empty cache just escalates, indistinguishable from a cold one.
		//
		// Derived, not fixed: see DefaultAdmitScore (and Load, for a config file that
		// replaces the tiers). Validate rejects an explicit value no tier can reach.
		CacheAdmitAt: 0, // set by Default(), from the tiers just defined
		ShadowRate:   0.05,

		ThresholdsPath: "thresholds.json",
	}
}

func defaultCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return d + "/go-cascade"
	}
	return ".go-cascade-cache"
}

// Validate rejects contradictory knob settings.
func (c *Config) Validate() error {
	if len(c.Tiers) == 0 {
		return errors.New("no tiers configured")
	}
	if c.Alpha > 0 && c.Budget > 0 {
		return errors.New("set --alpha or --budget, not both: they are dual forms of the same constraint")
	}
	if c.Alpha <= 0 && c.Budget <= 0 {
		return errors.New("one of --alpha or --budget must be set")
	}
	if c.Alpha < 0 || c.Alpha >= 1 {
		return fmt.Errorf("alpha must be in (0,1), got %v", c.Alpha)
	}
	for i := 1; i < len(c.Tiers); i++ {
		if c.Tiers[i].InPerMTok < c.Tiers[i-1].InPerMTok {
			return fmt.Errorf("tier %q is cheaper than %q: tiers must be ordered cheapest first",
				c.Tiers[i].Name, c.Tiers[i-1].Name)
		}
	}
	// Invariant #3: a cascade-level planner shapes the code every tier submits, so
	// it is a code author and must differ from the oracle's author, exactly like a
	// tier planner. Reject equality up front rather than flagging it at accept time.
	if c.PlannerModel != "" && c.PlannerModel == c.TestModel {
		return fmt.Errorf("planner_model equals test_model %q; the cascade planner "+
			"authors the code and must differ from the oracle's author (invariant #3)",
			c.TestModel)
	}
	for i, t := range c.Tiers {
		if t.Samples < 1 {
			return fmt.Errorf("tier %d (%s): samples must be >= 1", i, t.Name)
		}
		// Invariant #3: a two-stage tier's planner shapes the submitted code, so
		// it is a code author and must differ from the oracle's author. Equal to
		// TestModel is not a config error the run should silently proceed past —
		// it contaminates every record — so reject it up front rather than only
		// flagging it at accept time.
		if t.PlannerModel != "" && t.PlannerModel == c.TestModel {
			return fmt.Errorf("tier %d (%s): planner_model equals test_model %q; "+
				"the planner authors the code and must differ from the oracle's author (invariant #3)",
				i, t.Name, c.TestModel)
		}
		// A cascade-level planner and a tier-level planner are two independent plan
		// sources. Running both would draw two plans and double-charge, and it is
		// never what the caller wants — a cascade planner already threads its plan
		// into this tier. Reject the combination so cost attribution stays clean.
		if c.PlannerModel != "" && t.PlannerModel != "" {
			return fmt.Errorf("tier %d (%s): has its own planner_model while a "+
				"cascade-level planner_model is also set; use one plan source, not both",
				i, t.Name)
		}
	}
	return c.checkAdmissionReachable()
}

// MaxAttainableScore is the largest routing score this config can ever produce:
// the Wilson lower bound of a *unanimous* tier, maximised over tiers.
//
// The routing score is a lower confidence bound, not the raw cluster mass
// (invariant #9), so it is strictly below 1 at every finite sample count and is
// bounded by the widest fan-out in the cascade. That makes any threshold
// comparison against it checkable in advance, rather than only observable as an
// event that never happens.
func (c *Config) MaxAttainableScore() float64 {
	best := 0.0
	for _, t := range c.Tiers {
		if s := cluster.UnanimousScore(t.Samples); s > best {
			best = s
		}
	}
	return best
}

// DefaultAdmitScore is unanimity at the *narrowest* fan-out in the cascade.
//
// The tempting choice is the widest fan-out, but it is wrong, and the way it is
// wrong is instructive. Acceptance most often lands on the *final* tier — which is
// both the narrowest (typically one sample) and the one with no threshold at all,
// by construction (invariant #6). Keying admission to the widest tier's ceiling
// therefore admits nothing whenever the cascade escalates all the way, which is
// exactly the case where a cached solution would have paid for itself. Observed on
// the mock cascade: tiers [5,2,1] escalate to "large" and accept at 0.2698, against
// a widest-fan-out ceiling of 0.6488.
//
// So the floor over tiers is the only setting under which every tier can admit on
// unanimity. Admitting on thin evidence is safe here in a way it would not be on
// the acceptance path: a retrieved solution is re-executed against the new query's
// tests (invariant #5), so a weak admission costs one wasted verification, never a
// wrong answer. Retrieval quality is a cost question, not a risk one.
//
// The corollary is worth stating plainly, because README once claimed otherwise:
// admission cannot be uniformly "stricter than acceptance". The final tier accepts
// unconditionally, so nothing is stricter than that.
func (c *Config) DefaultAdmitScore() float64 {
	narrowest := 0
	for _, t := range c.Tiers {
		if narrowest == 0 || t.Samples < narrowest {
			narrowest = t.Samples
		}
	}
	return cluster.UnanimousScore(narrowest)
}

// checkAdmissionReachable rejects a config whose cache_admit_score exceeds the
// highest routing score the cascade can attain, because the solutions layer of
// the cache would then never be written and arm zero could never hit.
//
// This is not hypothetical, and it is why the default is now derived rather than
// fixed. Every config in examples/bench uses a fan-out of 1-5, whose unanimous
// Wilson bounds are 0.2698-0.6488, against the former default cache_admit_score
// of 0.90 — so the PutSolution call in Router.finish was unreachable in all of
// them and the arm-zero *solutions* layer was dead in every live experiment in
// results/. Nothing failed: an empty cache simply escalates, which is
// indistinguishable from a cold one. The spec and failure layers are unaffected —
// they are keyed exactly and not score-gated — which is why the measured
// spec-cache saving is real while solution reuse was not.
//
// Rejecting up front rather than warning is deliberate and matches the
// invariant #3 checks above: a silently unreachable threshold turns §5.5(4)'s
// absorption dial into a no-op, and a measurement taken through it would report
// "no §2.9 effect" from a configuration that cannot exhibit one. Only an
// *explicit* over-tight value can now reach this error, since an unset one is
// derived from the tiers.
func (c *Config) checkAdmissionReachable() error {
	// A disabled cache has no admission to reach, and an ungated one admits
	// everything; neither is the misconfiguration this guards against.
	if c.CacheDir == "" || c.CacheAdmitAt <= 0 {
		return nil
	}
	best := c.MaxAttainableScore()
	if c.CacheAdmitAt <= best {
		return nil
	}
	widest := 0
	for _, t := range c.Tiers {
		if t.Samples > widest {
			widest = t.Samples
		}
	}
	// Truncate rather than round the suggested replacement. best is an irrational
	// bound (0.4249871... at n=2), and %.3f would round it *up* to 0.425 — a value
	// that fails this very check, so the remedy the error suggests would not work.
	suggest := math.Floor(best*1000) / 1000
	return fmt.Errorf("cache_admit_score %v exceeds the highest attainable routing score %.4f "+
		"(widest fan-out is %d samples, and the score is a Wilson lower bound, not raw cluster mass "+
		"— invariant #9), so no solution could ever be admitted and arm zero would never hit. Lower "+
		"cache_admit_score to <= %.3f, raise samples to >= %d in some tier, or set cache_dir to \"\" "+
		"to disable the cache deliberately",
		c.CacheAdmitAt, best, widest, suggest, cluster.MinSamplesFor(c.CacheAdmitAt))
}

// TwoStage reports whether a single planner call runs for the whole cascade.
func (c *Config) TwoStage() bool { return c.PlannerModel != "" }

// PlannerCost prices the one cascade-level planner call. It uses the cascade
// planner's own per-MTok prices when set, otherwise tier 0's coder prices — the
// tier the single plan charge is attributed to for offline replay.
func (c *Config) PlannerCost(inTok, outTok int) float64 {
	in, out := c.PlannerInPerMTok, c.PlannerOutPerMTok
	if in == 0 && len(c.Tiers) > 0 {
		in = c.Tiers[0].InPerMTok
	}
	if out == 0 && len(c.Tiers) > 0 {
		out = c.Tiers[0].OutPerMTok
	}
	return float64(inTok)/1e6*in + float64(outTok)/1e6*out
}

// Load reads a JSON config file over the defaults.
//
// cache_admit_score is derived, not a constant: the default is unanimity for the
// widest fan-out, so a file that replaces "tiers" without naming an admit score
// gets the ceiling for *its* tiers rather than the one for Default()'s. Unmarshal
// alone cannot express that — an absent key and an explicit 0 are the same zero
// value — so the field is zeroed before decoding and re-derived if still zero.
func Load(path string) (*Config, error) {
	c := Default()
	c.CacheAdmitAt = 0
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.CacheAdmitAt == 0 {
		c.CacheAdmitAt = c.DefaultAdmitScore()
	}
	return c, nil
}
