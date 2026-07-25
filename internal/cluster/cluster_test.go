package cluster

import (
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/verify"
)

func ok(tests map[string]bool, complexity int) *verify.Report {
	return &verify.Report{OK: true, Tests: tests, Static: &verify.Static{MaxComplexity: complexity}}
}

func refuted(at verify.Stage) *verify.Report {
	return &verify.Report{OK: false, FailedAt: at}
}

// The point of clustering on behaviour rather than text: two differently
// written but observably identical solutions must land in one class.
func TestBehaviourIgnoresSourceText(t *testing.T) {
	pass := map[string]bool{"TestV_A": true, "TestV_B": true}
	a := Candidate{Index: 0, Source: "package solution // one way", Report: ok(pass, 3)}
	b := Candidate{Index: 1, Source: "package solution // a totally different spelling", Report: ok(pass, 5)}
	if Behaviour(a) != Behaviour(b) {
		t.Error("identical observed behaviour produced different cluster keys")
	}

	diff := Candidate{Index: 2, Report: ok(map[string]bool{"TestV_A": true, "TestV_B": false}, 3)}
	if Behaviour(a) == Behaviour(diff) {
		t.Error("different observed behaviour collapsed into one cluster")
	}
}

// Refuted candidates must not pool into a fake majority.
func TestRefutedCandidatesSeparateByStage(t *testing.T) {
	x := Candidate{Index: 0, Report: refuted(verify.StageTypes)}
	y := Candidate{Index: 1, Report: refuted(verify.StageTest)}
	if Behaviour(x) == Behaviour(y) {
		t.Error("candidates refuted at different stages share a cluster key")
	}
}

func TestGroupAndScore(t *testing.T) {
	pass := map[string]bool{"TestV_A": true}
	cands := []Candidate{
		{Index: 0, Source: "aaa", Report: ok(pass, 8)},
		{Index: 1, Source: "b", Report: ok(pass, 2)}, // simplest: should represent
		{Index: 2, Source: "ccc", Report: ok(pass, 9)},
		{Index: 3, Report: refuted(verify.StageTypes)},
	}
	cs := Group(cands)
	if len(cs) != 2 {
		t.Fatalf("want 2 clusters, got %d", len(cs))
	}
	if !cs[0].Verified {
		t.Error("verified clusters must sort first")
	}
	score, winner := Score(cs)
	if cs[0].Mass != 0.75 {
		t.Errorf("raw mass = %v, want 0.75", cs[0].Mass)
	}
	// The reported score is a lower bound on that mass, so it must sit below it.
	if score >= cs[0].Mass || score <= 0 {
		t.Errorf("score %v should be a positive lower bound on mass %v", score, cs[0].Mass)
	}
	if winner.Rep != 1 {
		t.Errorf("representative = %d, want the least complex candidate (1)", winner.Rep)
	}
}

// Nothing surviving is a sound refutation of the whole sample, not a
// low-confidence answer, and Score must express that as zero.
func TestScoreZeroWhenNothingSurvives(t *testing.T) {
	cands := []Candidate{
		{Index: 0, Report: refuted(verify.StageBuild)},
		{Index: 1, Report: refuted(verify.StageTest)},
	}
	score, winner := Score(Group(cands))
	if score != 0 || winner != nil {
		t.Errorf("score = %v winner = %v; want 0 and nil", score, winner)
	}
}

// The property that makes thresholds meaningful across tiers with different
// sample counts: the same proportion from more samples is stronger evidence.
func TestScoreRewardsMoreEvidenceAtEqualProportion(t *testing.T) {
	pass := map[string]bool{"TestV_A": true}
	mk := func(n int) float64 {
		var cands []Candidate
		for i := range n {
			cands = append(cands, Candidate{Index: i, Report: ok(pass, 1)})
		}
		s, _ := Score(Group(cands))
		return s
	}
	two, five, ten := mk(2), mk(5), mk(10)
	if !(two < five && five < ten) {
		t.Errorf("unanimity should strengthen with n: 2->%.3f 5->%.3f 10->%.3f", two, five, ten)
	}
	if two >= 0.5 {
		t.Errorf("unanimity among 2 samples scored %.3f; that is not enough evidence to accept on", two)
	}
	t.Logf("unanimous cluster: n=2 -> %.3f, n=5 -> %.3f, n=10 -> %.3f", two, five, ten)
}

func TestGroupEmpty(t *testing.T) {
	if cs := Group(nil); cs != nil {
		t.Errorf("Group(nil) = %v, want nil", cs)
	}
}
