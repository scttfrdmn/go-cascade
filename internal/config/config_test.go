package config

import (
	"encoding/json"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/cluster"
)

// The routing score is a Wilson lower bound, not raw cluster mass (invariant #9),
// so the ceiling for a unanimous tier is well below 1 and grows only with n. These
// are the values every admission threshold has to live under; pin them so a change
// to the bound or to z surfaces here rather than as a cache that stops filling.
func TestUnanimityCeilingIsFarBelowOne(t *testing.T) {
	for _, c := range []struct {
		n    int
		want float64
	}{
		{1, 0.2698},
		{2, 0.4249},
		{3, 0.5257},
		{5, 0.6488},
		{25, 0.9023},
	} {
		got := cluster.UnanimousScore(c.n)
		if math.Abs(got-c.want) > 5e-4 {
			t.Errorf("UnanimousScore(%d) = %.4f, want ~%.4f", c.n, got, c.want)
		}
		if got >= 1 {
			t.Errorf("UnanimousScore(%d) = %v, must be < 1: a lower bound never reaches certainty", c.n, got)
		}
	}
}

func TestMaxAttainableScoreTakesTheWidestFanOut(t *testing.T) {
	c := &Config{Tiers: []Tier{{Samples: 1}, {Samples: 5}, {Samples: 2}}}
	if got, want := c.MaxAttainableScore(), cluster.UnanimousScore(5); got != want {
		t.Errorf("MaxAttainableScore = %v, want the n=5 ceiling %v", got, want)
	}
}

// The whole point of the guard: an admit score above the ceiling means PutSolution
// can never run, so the solutions layer is dead and arm zero can never hit. That
// must be a config error, not a silently empty cache.
func TestUnreachableAdmitScoreIsRejected(t *testing.T) {
	c := Default()
	c.Tiers = []Tier{{Name: "only", Samples: 2, RepairDepth: 1}}
	c.CacheDir = t.TempDir()
	c.CacheAdmitAt = 0.90

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted cache_admit_score 0.90 against a 2-sample fan-out (ceiling 0.4249); admission would be unreachable")
	}
	msg := err.Error()
	for _, want := range []string{"cache_admit_score", "invariant #9", "raise samples to >= 25"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message lacks %q; got: %s", want, msg)
		}
	}
}

// The remedy an error suggests has to actually work. best is irrational (0.4249871
// at n=2) and rounding it to 3 places goes *up*, to a value that fails the same
// check — so the suggestion is truncated. Feed it back in and it must validate.
func TestSuggestedAdmitScoreActuallyValidates(t *testing.T) {
	for _, n := range []int{1, 2, 3, 5, 8} {
		c := Default()
		c.Tiers = []Tier{{Name: "only", Samples: n, RepairDepth: 1}}
		c.CacheDir = t.TempDir()
		c.CacheAdmitAt = 0.99

		err := c.Validate()
		if err == nil {
			t.Fatalf("n=%d: 0.99 should be unreachable", n)
		}
		var suggested float64
		if _, e := fmtScan(err.Error(), &suggested); e != nil {
			t.Fatalf("n=%d: could not read the suggested value out of %q: %v", n, err.Error(), e)
		}
		c.CacheAdmitAt = suggested
		if err := c.Validate(); err != nil {
			t.Errorf("n=%d: the value the error suggested (%v) does not validate: %v", n, suggested, err)
		}
	}
}

// fmtScan pulls the "Lower cache_admit_score to <= X" value out of the diagnostic.
func fmtScan(msg string, out *float64) (int, error) {
	const marker = "Lower cache_admit_score to <= "
	i := strings.Index(msg, marker)
	if i < 0 {
		return 0, errNoMarker
	}
	rest := msg[i+len(marker):]
	j := strings.IndexByte(rest, ',')
	if j < 0 {
		j = len(rest)
	}
	return 1, json.Unmarshal([]byte(strings.TrimSpace(rest[:j])), out)
}

var errNoMarker = errStr("no suggestion marker in the error message")

type errStr string

func (e errStr) Error() string { return string(e) }

// A disabled cache has no admission to reach, and an ungated one admits
// everything. Neither is the misconfiguration the guard is for.
func TestAdmissionGuardSkipsDisabledAndUngatedCaches(t *testing.T) {
	base := func() *Config {
		c := Default()
		c.Tiers = []Tier{{Name: "only", Samples: 1, RepairDepth: 1}}
		c.CacheAdmitAt = 0.99
		return c
	}
	c := base()
	c.CacheDir = ""
	if err := c.Validate(); err != nil {
		t.Errorf("an unreachable admit score with the cache disabled should be fine: %v", err)
	}
	c = base()
	c.CacheDir = t.TempDir()
	c.CacheAdmitAt = 0
	if err := c.Validate(); err != nil {
		t.Errorf("an ungated cache should be fine: %v", err)
	}
}

// The default must be reachable from EVERY tier, not merely from the widest one.
// Acceptance most often lands on the final tier, which is both the narrowest and
// the one with no threshold (invariant #6) — so a default keyed to the widest
// fan-out admits nothing on exactly the fully-escalated queries a cache would pay
// off on. Observed on the mock cascade: tiers [5,2,1] escalate to "large" and
// accept at 0.2698 against a widest-fan-out ceiling of 0.6488.
func TestDefaultAdmitScoreIsReachableFromEveryTier(t *testing.T) {
	c := Default()
	if c.CacheAdmitAt <= 0 {
		t.Fatalf("Default cache_admit_score = %v; a zero admits everything and defeats the point", c.CacheAdmitAt)
	}
	for _, tier := range c.Tiers {
		if ceiling := cluster.UnanimousScore(tier.Samples); c.CacheAdmitAt > ceiling {
			t.Errorf("tier %q (%d samples, ceiling %.4f) can never admit at cache_admit_score %.4f",
				tier.Name, tier.Samples, ceiling, c.CacheAdmitAt)
		}
	}
	if got, want := c.CacheAdmitAt, c.DefaultAdmitScore(); got != want {
		t.Errorf("Default cache_admit_score = %v, want the narrowest tier's ceiling %v", got, want)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Default() must validate: %v", err)
	}
}

// DefaultAdmitScore takes the floor over tiers, not the max — regardless of the
// order the tiers happen to appear in.
func TestDefaultAdmitScoreTakesTheNarrowestFanOut(t *testing.T) {
	for _, tiers := range [][]Tier{
		{{Samples: 5}, {Samples: 2}, {Samples: 1}},
		{{Samples: 1}, {Samples: 5}},
		{{Samples: 3}, {Samples: 3}},
	} {
		c := &Config{Tiers: tiers}
		narrowest := tiers[0].Samples
		for _, tr := range tiers {
			if tr.Samples < narrowest {
				narrowest = tr.Samples
			}
		}
		if got, want := c.DefaultAdmitScore(), cluster.UnanimousScore(narrowest); got != want {
			t.Errorf("tiers %v: DefaultAdmitScore = %v, want the n=%d ceiling %v", tiers, got, narrowest, want)
		}
	}
}

// A config file that replaces "tiers" without naming an admit score must get the
// ceiling for ITS tiers, not the one baked into Default(). Load zeroes the field
// before decoding precisely so an absent key is distinguishable here.
func TestLoadDerivesAdmitScoreFromTheFilesOwnTiers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	// Every tier here is 4 samples, so the file's own narrowest (4) differs from
	// Default()'s narrowest (1). Without the re-derivation in Load the result would
	// be the n=1 ceiling and this would catch it.
	body := `{"tiers":[
	  {"name":"small","model_id":"m1","input_usd_per_mtok":1,"output_usd_per_mtok":5,"samples":4,"repair_depth":1},
	  {"name":"mid","model_id":"m2","input_usd_per_mtok":3,"output_usd_per_mtok":15,"samples":4,"repair_depth":1}
	],"test_model":"oracle"}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.CacheAdmitAt, cluster.UnanimousScore(4); got != want {
		t.Errorf("cache_admit_score = %v, want the n=4 ceiling %v (Default()'s would be %v)",
			got, want, cluster.UnanimousScore(1))
	}
	if err := c.Validate(); err != nil {
		t.Errorf("a file that names no admit score must validate: %v", err)
	}
}

// An explicit value is honoured, including one below the ceiling.
func TestLoadKeepsAnExplicitAdmitScore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	body := `{"cache_admit_score":0.3,"test_model":"oracle"}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.CacheAdmitAt != 0.3 {
		t.Errorf("cache_admit_score = %v, want the explicit 0.3", c.CacheAdmitAt)
	}
}

// Every config we ship must validate. This is the regression that motivated the
// guard: all nine shipped configs inherited an admit score of 0.90 against
// fan-outs of 1-5, so none of them could ever admit a solution and no test said so.
func TestShippedConfigsValidateAndCanAdmit(t *testing.T) {
	// Walk rather than glob: filepath.Glob has no "**", so a pattern would silently
	// stop matching if a config moved a directory deeper — and a test that finds
	// nothing passes.
	var paths []string
	err := filepath.WalkDir("../../examples", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "config") && strings.HasSuffix(d.Name(), ".json") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 9 {
		t.Fatalf("found only %d shipped configs (%v); expected at least the 9 that exist", len(paths), paths)
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			c, err := Load(p)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if err := c.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if c.CacheDir == "" {
				return // deliberately cacheless
			}
			if c.CacheAdmitAt > c.MaxAttainableScore() {
				t.Errorf("cache_admit_score %v exceeds the attainable %v, so the solutions layer is dead",
					c.CacheAdmitAt, c.MaxAttainableScore())
			}
		})
	}
}
