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
	"os"
	"time"
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

		CacheDir:     defaultCacheDir(),
		CacheTopK:    5,
		CacheMinSim:  0.35,
		CacheAdmitAt: 0.90,
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
	}
	return nil
}

// Load reads a JSON config file over the defaults.
func Load(path string) (*Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}
