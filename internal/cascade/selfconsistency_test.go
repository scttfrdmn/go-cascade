package cascade

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/model"
)

// The duplicated const is only safe while it matches. calibrate cannot import
// cascade (the dependency must point one way), so nothing but this test keeps the
// two in step — and a drift would make the free feasibility gate flag a different
// set of problems degenerate than the paid sampling pass does, which is the one
// thing the gate exists to prevent.
func TestMinVoteMatchesCalibrate(t *testing.T) {
	if minVote != calibrate.MinVote {
		t.Fatalf("cascade.minVote = %d but calibrate.MinVote = %d; the feasibility gate and the "+
			"sampling arm would disagree about which rows are degenerate", minVote, calibrate.MinVote)
	}
}

// nonceRecorder captures the "(sample N)" nonce of every coder prompt alongside
// the request seed. On Bedrock the nonce IS the sample-diversity mechanism (no
// seed is exposed for Claude), so it is the thing worth asserting on.
type nonceRecorder struct {
	inner  model.Provider
	mu     sync.Mutex
	nonces []int
	seeds  []int
}

func (p *nonceRecorder) Name() string { return p.inner.Name() }

func (p *nonceRecorder) Generate(ctx context.Context, req model.Request) (*model.Response, error) {
	if req.Purpose == model.PurposeCode && len(req.Messages) > 0 {
		text := req.Messages[len(req.Messages)-1].Text
		if i := strings.LastIndex(text, "(sample "); i >= 0 {
			var n int
			if _, err := fmt.Sscanf(text[i:], "(sample %d)", &n); err == nil {
				p.mu.Lock()
				p.nonces = append(p.nonces, n)
				p.seeds = append(p.seeds, req.Seed)
				p.mu.Unlock()
			}
		}
	}
	return p.inner.Generate(ctx, req)
}

// The bug this guards against was written and caught before it shipped: a second
// batch that restarted its seeds at 0 would re-ask "(sample 0)" at the same
// temperature, so the "extra" candidates would be redraws of the first batch —
// inflating a self-consistency majority with duplicates it did not earn. Since
// Bedrock exposes no seed for Claude, the prompt nonce is the ONLY thing making
// two draws differ, so this is not a cosmetic property.
func TestSampleTierNOffsetsTheDiversityNonce(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	rec := &nonceRecorder{inner: model.Mock{}}
	r, err := New(cfg, rec, nil)
	if err != nil {
		t.Skipf("cannot build router (no go toolchain?): %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	ctx := context.Background()
	spec, err := r.spec(ctx, seqProblem, "", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.sampleN(ctx, 0, seqProblem, spec, "", 2, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := r.sampleN(ctx, 0, seqProblem, spec, "", 2, 2); err != nil {
		t.Fatal(err)
	}

	rec.mu.Lock()
	nonces, seeds := rec.nonces, rec.seeds
	rec.mu.Unlock()
	if len(nonces) != 4 {
		t.Fatalf("recorded %d coder prompts, want 4", len(nonces))
	}
	seen := map[int]bool{}
	for _, n := range nonces {
		if seen[n] {
			t.Fatalf("nonce %d was asked twice across the two batches (%v); the second batch is "+
				"redrawing the first and a text vote over these would agree by construction",
				n, nonces)
		}
		seen[n] = true
	}
	// The request seed must track the nonce too: on a provider that honours seeds,
	// a fixed seed would defeat the diversity the nonce stands in for.
	for i := range nonces {
		if seeds[i] != nonces[i] {
			t.Errorf("prompt nonce %d but request seed %d; the two diversity channels disagree",
				nonces[i], seeds[i])
		}
	}
}

// End to end on the mock: the fan-out must come from the budget, the spend must
// not exceed it, and a budget too small to seat a plurality must be flagged rather
// than reported as a self-consistency result.
func TestSelfConsistencyFanoutComesFromTheBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r := newRouter(t, cfg, nil)
	ctx := context.Background()

	// Learn this problem's per-sample cost the way the arm does, so the budgets
	// below are stated in units of samples rather than dollars guessed at.
	spec, err := r.spec(ctx, seqProblem, "", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	probe, err := r.sampleN(ctx, 0, seqProblem, spec, "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if probe.spent <= 0 {
		t.Skip("the provider reports zero cost, so no budget can be matched")
	}

	for _, tc := range []struct {
		name       string
		samples    int
		degenerate bool
	}{
		{"below a plurality", 2, true},
		{"a real vote", 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A half-unit over, so floor() lands exactly on tc.samples.
			budget := probe.spent * (float64(tc.samples) + 0.5)
			obs, err := r.SelfConsistency(ctx, "p1", seqProblem, 0, budget)
			if err != nil {
				t.Fatal(err)
			}
			if obs.Skipped != "" {
				t.Skipf("arm skipped: %s", obs.Skipped)
			}
			if obs.Fanout != tc.samples {
				t.Errorf("fan-out %d, want %d for a budget of %.1f per-sample units",
					obs.Fanout, tc.samples, float64(tc.samples)+0.5)
			}
			if obs.Degenerate != tc.degenerate {
				t.Errorf("Degenerate = %v at fan-out %d, want %v (threshold %d)",
					obs.Degenerate, obs.Fanout, tc.degenerate, minVote)
			}
			// Matched means matched. Overshooting would make arm (e) better funded
			// than arm (b) and the comparison meaningless.
			if obs.SpentUSD > obs.BudgetUSD {
				t.Errorf("spent $%.6f against a budget of $%.6f; the arms are no longer cost-matched",
					obs.SpentUSD, obs.BudgetUSD)
			}
			// The spec is the shared oracle every arm is scored against, and arm (b)'s
			// recorded spend excludes it, so charging it here would shift the matched
			// budgets by a constant. The spec prompt is much larger than one cheap-tier
			// coder call, so a leak shows up as spend far above fan-out × unit cost.
			if want := float64(obs.Fanout) * probe.spent; math.Abs(obs.SpentUSD-want) > 0.5*want {
				t.Errorf("spent $%.6f, want about $%.6f (%d × the probe's $%.6f); a much larger "+
					"figure means the spec or plan phase is being charged to the sampling budget",
					obs.SpentUSD, want, obs.Fanout, probe.spent)
			}
			if obs.TextMass <= 0 || obs.TextMass > 1 {
				t.Errorf("text mass %v outside (0,1]", obs.TextMass)
			}
		})
	}
}

// refutingProvider makes every candidate fail the ladder, so §3.5's selector has
// nothing to pick. Wrapping the mock rather than adding a mock model ID keeps the
// stipulated defect distribution — which the mock's whole design rests on —
// untouched.
type refutingProvider struct{ inner model.Provider }

func (p refutingProvider) Name() string { return p.inner.Name() }

func (p refutingProvider) Generate(ctx context.Context, req model.Request) (*model.Response, error) {
	resp, err := p.inner.Generate(ctx, req)
	if err != nil || req.Purpose != model.PurposeCode {
		return resp, err
	}
	// A distinct nonce per sample, so the candidates are not byte-identical and the
	// abstention is not an artifact of a single-member candidate set.
	return &model.Response{
		Text:  fmt.Sprintf("```go\npackage solution\n\n// draw %d\nthis is not go\n```", req.Seed),
		Usage: resp.Usage,
	}, nil
}

// An abstention is not a wrong answer. When nothing survives the ladder §3.5's
// selector has nothing to pick, which is a sound refutation of the whole sample
// (invariant #4) and an escalation in a real cascade. Scoring it as incorrect
// would flatter the text vote, which is the arm this comparison exists to test.
func TestClusterAbstentionIsNotScoredAsWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r, err := New(cfg, refutingProvider{model.Mock{}}, nil)
	if err != nil {
		t.Skipf("cannot build router (no go toolchain?): %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	ctx := context.Background()

	spec, err := r.spec(ctx, seqProblem, "", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	probe, err := r.sampleN(ctx, 0, seqProblem, spec, "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if probe.spent <= 0 {
		t.Skip("the provider reports zero cost, so no budget can be matched")
	}
	obs, err := r.SelfConsistency(ctx, "p1", seqProblem, 0, probe.spent*3.5)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Skipped != "" {
		t.Skipf("arm skipped: %s", obs.Skipped)
	}
	if !obs.ClusterAbstained {
		t.Fatalf("a tier whose every candidate is refuted recorded no abstention: %+v", obs)
	}
	if obs.ClusterCorrect {
		t.Error("an abstention was recorded as a correct cluster choice")
	}
	if obs.Agreed {
		t.Error("Agreed was set with no cluster representative to agree with")
	}
	// The text vote still happens — self-consistency does not consult a verifier,
	// so it votes on refuted candidates too and is simply wrong. That asymmetry is
	// the measurement, so the row must not be silently dropped.
	if obs.TextMass <= 0 {
		t.Error("the text vote abstained as well; arm (e) must vote without consulting the ladder")
	}
	if obs.TextCorrect {
		t.Error("an unparseable candidate was scored correct")
	}
}
