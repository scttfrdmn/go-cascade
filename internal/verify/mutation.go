package verify

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"time"
)

// MutationScore estimates the quality of the oracle itself.
//
// Passing the tests only bounds risk to the extent the tests can distinguish a
// correct program from an incorrect one. Mutation score measures exactly that
// discrimination: it is the fraction of deliberately broken variants the suite
// rejects. The complement is the part of the residual risk that comes from the
// oracle rather than from the model, and it is what lets a cascade with an
// executable oracle certify below an LLM judge's noise floor.
type MutationScore struct {
	Generated int           `json:"generated"`
	Valid     int           `json:"valid"` // compiled and ran
	Killed    int           `json:"killed"`
	Score     float64       `json:"score"` // Killed / Valid
	Survivors []string      `json:"survivors,omitempty"`
	Elapsed   time.Duration `json:"elapsed"`
}

// mutation is one edit site.
type mutation struct {
	apply  func()
	revert func()
	desc   string
}

// swaps are operator substitutions that change behaviour without usually
// changing types, so most mutants compile.
var swaps = map[token.Token]token.Token{
	token.LSS: token.LEQ, token.LEQ: token.LSS,
	token.GTR: token.GEQ, token.GEQ: token.GTR,
	token.EQL: token.NEQ, token.NEQ: token.EQL,
	token.ADD: token.SUB, token.SUB: token.ADD,
	token.LAND: token.LOR, token.LOR: token.LAND,
}

// collectSites parses src and enumerates the mutation edit sites over its AST,
// returning the sites plus the FileSet and File needed to print each mutant.
// Shared by Mutate (which scores) and KilledMutants (which harvests wrong
// programs), so both use exactly the same operator set.
func collectSites(src string) ([]mutation, *token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}
	var sites []mutation
	ast.Inspect(f, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.BinaryExpr:
			if to, ok := swaps[t.Op]; ok {
				from := t.Op
				sites = append(sites, mutation{
					apply:  func() { t.Op = to },
					revert: func() { t.Op = from },
					desc:   fset.Position(t.OpPos).String() + ": " + from.String() + " -> " + to.String(),
				})
			}
		case *ast.IncDecStmt:
			from := t.Tok
			to := token.INC
			if from == token.INC {
				to = token.DEC
			}
			sites = append(sites, mutation{
				apply:  func() { t.Tok = to },
				revert: func() { t.Tok = from },
				desc:   fset.Position(t.TokPos).String() + ": " + from.String() + " -> " + to.String(),
			})
		case *ast.IfStmt:
			orig := t.Cond
			neg := &ast.UnaryExpr{Op: token.NOT, X: &ast.ParenExpr{X: orig}}
			sites = append(sites, mutation{
				apply:  func() { t.Cond = neg },
				revert: func() { t.Cond = orig },
				desc:   fset.Position(t.If).String() + ": negate condition",
			})
		}
		return true
	})
	return sites, fset, f, nil
}

// Mutate runs mutation analysis on src using both test partitions. budget caps
// the number of mutants; sites are sampled evenly so the estimate is not biased
// toward the top of the file.
func Mutate(ctx context.Context, r *Runner, src, visible, hidden string, budget int, timeout time.Duration) (*MutationScore, error) {
	start := time.Now()
	sites, fset, f, err := collectSites(src)
	if err != nil {
		return nil, err
	}

	ms := &MutationScore{Generated: len(sites)}
	if len(sites) == 0 {
		ms.Elapsed = time.Since(start)
		return ms, nil
	}

	chosen := sample(len(sites), budget)
	ws, err := r.NewWorkspace(src, visible, hidden)
	if err != nil {
		return nil, err
	}
	defer ws.Remove() //nolint:errcheck // scratch dir

	for _, i := range chosen {
		select {
		case <-ctx.Done():
			ms.Elapsed = time.Since(start)
			return ms, nil
		default:
		}
		m := sites[i]
		m.apply()
		var buf bytes.Buffer
		perr := printer.Fprint(&buf, fset, f)
		m.revert()
		if perr != nil {
			continue
		}
		if err := ws.WriteSolution(buf.String()); err != nil {
			continue
		}
		// A mutant that fails to build is not evidence about the test suite,
		// so it is excluded rather than counted as killed.
		if b := ws.run(ctx, 60*time.Second, false, "build", "./..."); b.Err != nil {
			continue
		}
		ms.Valid++
		res := ws.run(ctx, timeout, true, "test", "-count=1", "./...")
		if res.Err != nil || res.TimedOut {
			ms.Killed++ // the suite noticed
		} else {
			ms.Survivors = append(ms.Survivors, m.desc)
		}
	}
	if ms.Valid > 0 {
		ms.Score = float64(ms.Killed) / float64(ms.Valid)
	}
	ms.Elapsed = time.Since(start)
	return ms, nil
}

// KilledMutants returns up to n mutant sources of src that COMPILE and are
// KILLED by the test suite -- i.e. programs that are provably wrong by execution
// yet close to a correct solution (a single operator swap, increment flip, or
// condition negation away). It is the seed generator for the judge dangerous-mode
// experiment: a reading-only judge must rule on these known-wrong candidates, so
// a PASS is an unambiguous false acceptance (η_fa).
//
// Only mutants that build are considered (a non-compiling mutant is not a
// plausible candidate), and only those the suite actually kills are returned (a
// surviving mutant might be behaviourally equivalent, so it is not provably
// wrong). Each returned source is paired with the mutation description.
func KilledMutants(ctx context.Context, r *Runner, src, visible, hidden string, n int, timeout time.Duration) ([]KilledMutant, error) {
	sites, fset, f, err := collectSites(src)
	if err != nil {
		return nil, err
	}
	if len(sites) == 0 || n <= 0 {
		return nil, nil
	}
	ws, err := r.NewWorkspace(src, visible, hidden)
	if err != nil {
		return nil, err
	}
	defer ws.Remove() //nolint:errcheck // scratch dir

	var out []KilledMutant
	for _, i := range sample(len(sites), len(sites)) {
		if len(out) >= n {
			break
		}
		select {
		case <-ctx.Done():
			return out, nil
		default:
		}
		m := sites[i]
		m.apply()
		var buf bytes.Buffer
		perr := printer.Fprint(&buf, fset, f)
		m.revert()
		if perr != nil {
			continue
		}
		mutant := buf.String()
		if err := ws.WriteSolution(mutant); err != nil {
			continue
		}
		if b := ws.run(ctx, 60*time.Second, false, "build", "./..."); b.Err != nil {
			continue // does not compile: not a plausible candidate
		}
		res := ws.run(ctx, timeout, true, "test", "-count=1", "./...")
		if res.Err != nil || res.TimedOut {
			out = append(out, KilledMutant{Source: mutant, Desc: m.desc}) // killed => provably wrong
		}
	}
	return out, nil
}

// KilledMutant is a compiling, test-refuted variant of a solution.
type KilledMutant struct {
	Source string
	Desc   string
}

// sample picks up to budget evenly spaced indices from [0,n).
func sample(n, budget int) []int {
	if budget <= 0 || budget >= n {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, budget)
	step := float64(n) / float64(budget)
	for i := range budget {
		out = append(out, int(float64(i)*step))
	}
	return out
}
