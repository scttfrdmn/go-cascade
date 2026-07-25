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

// Mutate runs mutation analysis on src using both test partitions. budget caps
// the number of mutants; sites are sampled evenly so the estimate is not biased
// toward the top of the file.
func Mutate(ctx context.Context, r *Runner, src, visible, hidden string, budget int, timeout time.Duration) (*MutationScore, error) {
	start := time.Now()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
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
