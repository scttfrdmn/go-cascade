// Package cache implements arm zero of the cascade.
//
// The cache is not a separate system bolted in front of the router; it is a
// tier with cost ~0 that participates in the same accept/escalate rule. Because
// its gate is an executed verification rather than a similarity prediction, it
// adds no term to the risk budget: at equal risk the cascade with a cache is
// never more expensive than the one without, since the gate can always reject.
//
// Three layers are kept:
//
//	solutions  prior solutions, retrieved by similarity and re-verified
//	specs      generated API contracts and test partitions, keyed exactly
//	failures   refuted canonical forms, fed forward as negative constraints
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"
)

// Entry is a stored solution.
type Entry struct {
	ID            string    `json:"id"`
	Problem       string    `json:"problem"`
	ProblemHash   string    `json:"problem_hash"`
	API           string    `json:"api"`
	Solution      string    `json:"solution"`
	CanonHash     string    `json:"canon_hash"`
	Tier          string    `json:"tier"`
	Score         float64   `json:"score"`
	MutationScore float64   `json:"mutation_score"`
	CreatedAt     time.Time `json:"created_at"`
}

// Spec is the cached contract for a problem.
type Spec struct {
	ProblemHash  string    `json:"problem_hash"`
	API          string    `json:"api"`
	VisibleTests string    `json:"visible_tests"`
	HiddenTests  string    `json:"hidden_tests"`
	Author       string    `json:"author"` // model that wrote the oracle
	CreatedAt    time.Time `json:"created_at"`
}

// Failure records a refuted canonical form.
type Failure struct {
	CanonHash string `json:"canon_hash"`
	Stage     string `json:"stage"`
	Summary   string `json:"summary"`
}

// Cache is a filesystem-backed store.
type Cache struct {
	dir      string
	disabled bool
}

// Open prepares the cache directories. An empty dir disables the cache, which
// is how the shadow stream bypasses it.
func Open(dir string) (*Cache, error) {
	if dir == "" {
		return &Cache{disabled: true}, nil
	}
	for _, sub := range []string{"solutions", "specs", "failures"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("prepare cache: %w", err)
		}
	}
	return &Cache{dir: dir}, nil
}

// Disabled reports whether this cache is a no-op.
func (c *Cache) Disabled() bool { return c.disabled }

func (c *Cache) path(sub, name string) string {
	return filepath.Join(c.dir, sub, name+".json")
}

// writeJSON writes atomically so a crash cannot leave a torn entry that would
// later be read back as a valid cached solution.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// PutSpec stores a generated contract.
func (c *Cache) PutSpec(s Spec) error {
	if c.disabled {
		return nil
	}
	s.CreatedAt = time.Now().UTC()
	return writeJSON(c.path("specs", s.ProblemHash), s)
}

// GetSpec returns a cached contract for an exact problem match.
func (c *Cache) GetSpec(problemHash string) (*Spec, bool) {
	if c.disabled {
		return nil, false
	}
	var s Spec
	if err := readJSON(c.path("specs", problemHash), &s); err != nil {
		return nil, false
	}
	return &s, true
}

// PutSolution admits a solution. Admission is deliberately stricter than the
// acceptance threshold: an entry that will be reused many times should clear a
// higher bar than one that is returned once.
func (c *Cache) PutSolution(e Entry) error {
	if c.disabled {
		return nil
	}
	if e.CanonHash == "" {
		h, err := CanonicalHash(e.Solution)
		if err != nil {
			return err
		}
		e.CanonHash = h
	}
	e.ID = e.ProblemHash + "-" + e.CanonHash
	e.CreatedAt = time.Now().UTC()
	return writeJSON(c.path("solutions", e.ID), e)
}

// Scored is a retrieval result.
type Scored struct {
	Entry
	Similarity float64
}

// Retrieve returns up to k prior solutions whose problem statements are at
// least minSim similar to this one, best first. Exact problem-hash matches are
// always ranked first.
//
// Nothing here is trusted: the caller re-runs the verifier ladder against the
// new query's tests before any of these can be returned.
func (c *Cache) Retrieve(problem string, k int, minSim float64) ([]Scored, error) {
	if c.disabled {
		return nil, nil
	}
	target := ProblemHash(problem)
	dir := filepath.Join(c.dir, "solutions")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []Scored
	for _, de := range ents {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		var e Entry
		if readJSON(filepath.Join(dir, de.Name()), &e) != nil {
			continue
		}
		sim := 1.0
		if e.ProblemHash != target {
			sim = Similarity(problem, e.Problem)
			if sim < minSim {
				continue
			}
		}
		out = append(out, Scored{Entry: e, Similarity: sim})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Similarity != out[j].Similarity {
			return out[i].Similarity > out[j].Similarity
		}
		return out[i].Score > out[j].Score
	})
	if k > 0 && len(out) > k {
		out = out[:k]
	}
	return out, nil
}

// AddFailure records a refuted canonical form so later tiers do not rediscover
// it. Duplicates are collapsed.
func (c *Cache) AddFailure(problemHash string, f Failure) error {
	if c.disabled {
		return nil
	}
	fs := c.Failures(problemHash)
	if slices.ContainsFunc(fs, func(x Failure) bool { return x.CanonHash == f.CanonHash }) {
		return nil
	}
	fs = append(fs, f)
	if len(fs) > 64 {
		fs = fs[len(fs)-64:]
	}
	return writeJSON(c.path("failures", problemHash), fs)
}

// Failures returns previously refuted forms for a problem.
func (c *Cache) Failures(problemHash string) []Failure {
	if c.disabled {
		return nil
	}
	var fs []Failure
	_ = readJSON(c.path("failures", problemHash), &fs)
	return fs
}

// Stats summarises cache contents.
type Stats struct {
	Solutions int `json:"solutions"`
	Specs     int `json:"specs"`
	Failures  int `json:"failures"`
}

// Stats counts entries.
func (c *Cache) Stats() Stats {
	var s Stats
	if c.disabled {
		return s
	}
	count := func(sub string) int {
		ents, err := os.ReadDir(filepath.Join(c.dir, sub))
		if err != nil {
			return 0
		}
		n := 0
		for _, e := range ents {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
				n++
			}
		}
		return n
	}
	s.Solutions, s.Specs, s.Failures = count("solutions"), count("specs"), count("failures")
	return s
}
