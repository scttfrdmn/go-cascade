// Package cluster groups candidate solutions by observed behaviour.
//
// Text-level agreement between samples is a weak signal: two spellings of the
// same algorithm look different, and two different algorithms can look alike.
// Executing the candidates and clustering on the resulting outcome vector is
// both more discriminative and, for Go, cheap enough to do on every sample.
// Cluster mass then serves as the routing score. It does not need to be a
// calibrated probability, only monotone in correctness; calibration is supplied
// downstream by the conformal procedure.
package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/scttfrdmn/go-cascade/internal/verify"
)

// Candidate is one sampled solution and its verification report.
type Candidate struct {
	Index   int
	Source  string
	Report  *verify.Report
	Elapsed int64
}

// Cluster is a behavioural equivalence class.
type Cluster struct {
	Key      string  `json:"key"`
	Members  []int   `json:"members"`
	Mass     float64 `json:"mass"`     // |members| / n
	Verified bool    `json:"verified"` // members survived the ladder
	Rep      int     `json:"representative"`
}

// Behaviour derives the clustering key for a candidate.
//
// Refuted candidates are keyed by the stage that refuted them, so all the
// broken ones do not collapse into a single misleading majority.
func Behaviour(c Candidate) string {
	if c.Report == nil {
		return "nil"
	}
	if !c.Report.OK {
		return "refuted:" + c.Report.FailedAt.String()
	}
	names := slices.Sorted(maps.Keys(c.Report.Tests))
	var b strings.Builder
	b.WriteString("ok:")
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte('=')
		b.WriteString(strconv.FormatBool(c.Report.Tests[n]))
		b.WriteByte(';')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "ok:" + hex.EncodeToString(sum[:8])
}

// Group partitions candidates into behavioural classes, ordered by descending
// mass with verified classes first.
func Group(cands []Candidate) []Cluster {
	if len(cands) == 0 {
		return nil
	}
	byKey := map[string]*Cluster{}
	for _, c := range cands {
		k := Behaviour(c)
		cl, ok := byKey[k]
		if !ok {
			cl = &Cluster{Key: k, Verified: c.Report != nil && c.Report.OK, Rep: c.Index}
			byKey[k] = cl
		}
		cl.Members = append(cl.Members, c.Index)
	}

	// Prefer the simplest representative among behaviourally identical
	// candidates: same observable behaviour, less code to maintain.
	byIndex := map[int]Candidate{}
	for _, c := range cands {
		byIndex[c.Index] = c
	}
	out := make([]Cluster, 0, len(byKey))
	for _, cl := range byKey {
		cl.Mass = float64(len(cl.Members)) / float64(len(cands))
		best, bestScore := cl.Members[0], 1<<30
		for _, m := range cl.Members {
			s := 1 << 29
			if r := byIndex[m].Report; r != nil && r.Static != nil {
				s = r.Static.MaxComplexity*1000 + len(byIndex[m].Source)/100
			}
			if s < bestScore {
				best, bestScore = m, s
			}
		}
		cl.Rep = best
		out = append(out, *cl)
	}
	slices.SortFunc(out, func(a, b Cluster) int {
		if a.Verified != b.Verified {
			if a.Verified {
				return -1
			}
			return 1
		}
		if d := len(b.Members) - len(a.Members); d != 0 {
			return d
		}
		return strings.Compare(a.Key, b.Key)
	})
	return out
}

// wilsonLCB is the one-sided Wilson lower confidence bound on a binomial
// proportion, at roughly 95% for z = 1.645.
func wilsonLCB(k, n int, z float64) float64 {
	if n <= 0 {
		return 0
	}
	fn := float64(n)
	p := float64(k) / fn
	z2 := z * z
	centre := p + z2/(2*fn)
	spread := z * math.Sqrt(p*(1-p)/fn+z2/(4*fn*fn))
	lcb := (centre - spread) / (1 + z2/fn)
	return math.Max(0, math.Min(1, lcb))
}

// Score is the routing statistic: a lower confidence bound on the mass of the
// largest verified behavioural class.
//
// The raw fraction will not do. Unanimity among two samples and unanimity among
// five are both mass 1.0, but they are not the same evidence, and a tier that
// always reports 1.0 gives a threshold nothing to bite on. Calibration on a
// real benchmark showed exactly that failure: a two-sample tier scored 1.0 on
// every problem while being wrong 39% of the time, and no threshold could
// certify. Bounding the proportion below makes the statistic monotone in
// evidence rather than in sample luck, so a tier that has not earned confidence
// escalates instead.
//
// Zero means nothing survived the ladder, which is a sound refutation of the
// whole sample rather than a low-confidence result.
func Score(cs []Cluster) (float64, *Cluster) {
	for i := range cs {
		if cs[i].Verified {
			n := 0
			for j := range cs {
				n += len(cs[j].Members)
			}
			return wilsonLCB(len(cs[i].Members), n, 1.645), &cs[i]
		}
	}
	return 0, nil
}

// UnanimousScore is the score a tier of n samples reports when every candidate
// lands in one verified cluster — the ceiling of Score for that fan-out.
//
// Because the statistic is a lower bound rather than raw mass, this ceiling is
// strictly below 1 and grows only with n: 0.270 at n=1, 0.425 at n=2, 0.649 at
// n=5. Any threshold above the ceiling is therefore unreachable *by
// construction*, which is checkable in advance instead of merely being an event
// that never fires. Config.Validate uses this to reject a cache_admit_score no
// fan-out in the cascade can satisfy.
func UnanimousScore(n int) float64 { return wilsonLCB(n, n, 1.645) }

// MinSamplesFor returns the smallest unanimous fan-out whose score reaches want,
// or 0 if no fan-out does. It exists to make the diagnostic for an unreachable
// threshold actionable: "raise samples to >= 25" rather than "this cannot work".
//
// The search is bounded because UnanimousScore is increasing in n and tends to 1;
// the cap is a guard against a want of 1.0, which no finite fan-out attains.
func MinSamplesFor(want float64) int {
	const cap = 100000
	for n := 1; n <= cap; n++ {
		if UnanimousScore(n) >= want {
			return n
		}
	}
	return 0
}
