package cascade

import (
	"fmt"
	"strings"
	"time"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cluster"
	"github.com/scttfrdmn/go-cascade/internal/verify"
)

// Cost separates the two currencies the cascade spends. They rank the ladder in
// opposite directions: dollars say verify hard and escalate rarely, latency
// says stop at the type stage and escalate at once.
type Cost struct {
	ModelUSD     float64       `json:"model_usd"`
	ComputeUSD   float64       `json:"compute_usd"`
	TotalUSD     float64       `json:"total_usd"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	VerifierTime time.Duration `json:"verifier_time"`
	ModelCalls   int           `json:"model_calls"`
}

func (c *Cost) addModel(in, out int, usd float64) {
	c.InputTokens += in
	c.OutputTokens += out
	c.ModelUSD += usd
	c.TotalUSD = c.ModelUSD + c.ComputeUSD
	c.ModelCalls++
}

func (c *Cost) addCompute(d time.Duration, usdPerCoreSec float64) {
	c.VerifierTime += d
	c.ComputeUSD += d.Seconds() * usdPerCoreSec
	c.TotalUSD = c.ModelUSD + c.ComputeUSD
}

// Action names a routing decision.
type Action string

// Routing decisions.
const (
	ActAccept   Action = "accept"
	ActRepair   Action = "repair"
	ActEscalate Action = "escalate"
	ActReject   Action = "reject"
)

// Step is one decision in the trace.
type Step struct {
	Stage      string            `json:"stage"`
	Tier       string            `json:"tier,omitempty"`
	Action     Action            `json:"action"`
	Score      float64           `json:"score,omitempty"`
	Threshold  float64           `json:"threshold,omitempty"`
	Clusters   []cluster.Cluster `json:"clusters,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Diagnostic string            `json:"diagnostic,omitempty"`
	CostSoFar  float64           `json:"cost_so_far_usd"`
	Elapsed    time.Duration     `json:"elapsed"`

	// TimedOut marks a step where some verifier stage was killed by the timeout
	// rather than refuted by the program. The decision recorded in Action stands
	// (invariant #4), but a timeout is the one refutation whose cause can be
	// external to the candidate, so a step carrying this flag may be reporting the
	// machine's load rather than the model's output.
	//
	// FORENSIC ONLY, like verify.StageResult.TimedOut, from which it is derived:
	// nothing on the acceptance path may branch on it. It is here so that
	// Router.record can carry it onto the calibration record.
	TimedOut bool `json:"timed_out,omitempty"`
}

// Result is what the router returns.
type Result struct {
	Problem    string  `json:"problem"`
	Solution   string  `json:"solution"`
	API        string  `json:"api"`
	AcceptedAt string  `json:"accepted_at"`
	Score      float64 `json:"score"`
	Solved     bool    `json:"solved"`

	Cost    Cost          `json:"cost"`
	Elapsed time.Duration `json:"elapsed"`
	Trace   []Step        `json:"trace"`

	// Certified is true only when thresholds came from a valid calibration
	// certificate. An uncalibrated run is still useful; it just carries no
	// guarantee and must not pretend otherwise.
	Certified   bool                   `json:"certified"`
	Certificate *calibrate.Certificate `json:"certificate,omitempty"`

	Mutation *verify.MutationScore `json:"mutation,omitempty"`
	Static   *verify.Static        `json:"static,omitempty"`

	// OracleContaminated is set when the model that wrote the tests also wrote
	// the accepted code. Such a run is excluded from calibration.
	OracleContaminated bool `json:"oracle_contaminated"`
	// Shadow marks a run routed past the cache for calibration purposes.
	Shadow bool `json:"shadow"`

	VisibleTests string `json:"visible_tests,omitempty"`
	HiddenTests  string `json:"hidden_tests,omitempty"`
}

// RiskStatement renders the honest bound: what was established and what was not.
func (r *Result) RiskStatement() string {
	var b strings.Builder
	if !r.Solved {
		return "no candidate survived the verifier ladder; nothing is claimed"
	}
	if r.Certified && r.Certificate != nil {
		fmt.Fprintf(&b, "risk <= %.3f with confidence %.2f (n=%d, %s)",
			r.Certificate.Alpha, 1-r.Certificate.Delta, r.Certificate.N, r.Certificate.Method)
	} else {
		b.WriteString("UNCERTIFIED: thresholds are priors, not a calibrated bound")
	}
	if r.Mutation != nil && r.Mutation.Valid > 0 {
		fmt.Fprintf(&b, "; oracle gap ~%.0f%% (mutation score %.2f over %d valid mutants)",
			100*(1-r.Mutation.Score), r.Mutation.Score, r.Mutation.Valid)
	}
	if r.OracleContaminated {
		b.WriteString("; ORACLE CONTAMINATED: tests and code share an author")
	}
	return b.String()
}
