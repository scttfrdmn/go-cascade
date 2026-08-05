package verify

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
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

	// DataRace is true when ThreadSanitizer actually reported a race, as opposed
	// to the mutant merely failing under `-race` (a wrong answer, or a timeout).
	// Experiment 28's first pass conflated the two and reported six "race seeds"
	// of which one — a deferred wg.Wait() on conc_safe_counter — produced no race
	// at all, just `got 0 want 400`. For the scar-free arm this is a hard filter
	// (see ScarFreeRaceKilledMutants); the deletion arm keeps the flag forensic so
	// the shared harvest stays comparable.
	DataRace bool

	// PlainRefuted is true when the ordinary test run (no -race) also refutes the
	// mutant. It is recorded, never filtered on: for a *race* seed it means the
	// defect is not reading-invisible after all — a reader sees a deterministically
	// wrong program — which is a property of the seed set that belongs in the
	// write-up rather than a reason to discard, since discarding on one arm only
	// would make the two arms' η_fa rates incomparable (raceKilledFrom's contract).
	PlainRefuted bool
}

// collectRaceSites finds statements whose deletion introduces a data race
// without breaking compilation: a `wg.Wait()` call (letting the function return
// while goroutines still write) or a mutex `Lock`/`Unlock`/`RLock`/`RUnlock`
// call (unguarding a shared access). Each site removes one statement.
//
// These are the operators the logic-mutation set (collectSites) cannot produce,
// and they target the one defect class observed to slip past the judge live: a
// race is invisible to a reader and caught only under `-race`.
func collectRaceSites(src string) ([]mutation, *token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}
	syncMethods := map[string]bool{
		"Wait": true, "Lock": true, "Unlock": true, "RLock": true, "RUnlock": true,
	}
	var sites []mutation
	// Walk every statement list and mark deletable sync-call statements. We edit
	// the enclosing block's Stmt slice, replacing the target with an empty
	// statement so positions of siblings are unaffected and the edit is reversible.
	ast.Inspect(f, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for idx, stmt := range block.List {
			exprStmt, ok := stmt.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := exprStmt.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !syncMethods[sel.Sel.Name] {
				continue
			}
			i, orig := idx, stmt
			method := sel.Sel.Name
			pos := fset.Position(exprStmt.Pos()).String()
			sites = append(sites, mutation{
				apply:  func() { block.List[i] = &ast.EmptyStmt{} },
				revert: func() { block.List[i] = orig },
				desc:   pos + ": delete ." + method + "()",
			})
		}
		return true
	})
	return sites, fset, f, nil
}

// exprText renders an expression back to source, used to compare two selector
// receivers by text. Textual identity is deliberately conservative: `mu` matches
// `mu` but not `s.mu`, so a pair operator never edits two calls that might be on
// different mutexes. A missed pair costs a seed candidate; a wrong pair would
// produce a mutant whose defect is not the one the description claims.
func exprText(fset *token.FileSet, x ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		return ""
	}
	return buf.String()
}

// syncCall extracts the receiver text, method name and selector from a statement
// that is a bare or deferred method call — `mu.Unlock()` and
// `defer mu.Unlock()` both yield ("mu", "Unlock", sel). The deferred form has to
// be recognised because `Lock(); defer Unlock()` is the dominant Go idiom, and an
// operator that only saw the bare form would skip most real critical sections.
func syncCall(fset *token.FileSet, stmt ast.Stmt) (recv, method string, sel *ast.SelectorExpr, ok bool) {
	var call *ast.CallExpr
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		call, ok = s.X.(*ast.CallExpr)
	case *ast.DeferStmt:
		call, ok = s.Call, s.Call != nil
	}
	if !ok || call == nil {
		return "", "", nil, false
	}
	sel, ok = call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", "", nil, false
	}
	return exprText(fset, sel.X), sel.Sel.Name, sel, true
}

// mutexTypeOf finds the `sync.Mutex` or `sync.RWMutex` type expression declaring
// the lock a receiver refers to, so the downgrade operator can co-mutate it to
// `sync.RWMutex` in the same edit. Returns nil when no such declaration is found.
//
// RWMutex is matched as well as Mutex, and not for symmetry: a source that
// already declares an RWMutex needs no type change but is still a valid site, and
// an earlier version that matched only `Mutex` would have skipped it. Rewriting
// RWMutex to RWMutex is a no-op, so one code path covers both.
//
// It matches on the receiver's final identifier (`c.mu` and `r.mu` both look for
// `mu`) across every `var` spec and struct field in the file, which is a
// heuristic: a file declaring two distinct mutexes both named `mu` could have the
// wrong one rewritten. That is acceptable *only* because the harvest filter is
// downstream — a mismatched rewrite either fails to compile or fails to race, and
// either way the mutant is discarded rather than counted. Do not lift this helper
// somewhere that lacks that backstop.
func mutexTypeOf(f *ast.File, recv string) *ast.SelectorExpr {
	name := recv
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	var found *ast.SelectorExpr
	consider := func(names []*ast.Ident, typ ast.Expr) {
		sel, ok := typ.(*ast.SelectorExpr)
		if !ok || found != nil {
			return
		}
		pkg, isIdent := sel.X.(*ast.Ident)
		if !isIdent || pkg.Name != "sync" ||
			(sel.Sel.Name != "Mutex" && sel.Sel.Name != "RWMutex") {
			return
		}
		for _, n := range names {
			if n.Name == name {
				found = sel
				return
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.ValueSpec:
			consider(d.Names, d.Type)
		case *ast.Field:
			consider(d.Names, d.Type)
		}
		return found == nil
	})
	return found
}

// clearPositions zeroes every position field in a subtree and returns a closure
// that restores them, so a reordering mutation can be printed without inheriting
// the original line numbers — and still be reverted, since these operators are
// applied and undone repeatedly against one shared AST.
//
// It exists because go/printer lays a block out from its statements' recorded
// source lines, not from their slice order. Swap two statements and the printer
// sees a line number that runs backwards, which it renders as a blank line at the
// mutation site. For an arm whose entire premise is that the defect leaves no
// mark on the page, a stray blank line exactly where the edit happened is a
// scar — and one that gofmt does not remove, because gofmt preserves author
// blank lines between statements rather than normalising them away. Verified:
// the shipped reordering operator emits one such line on hard_conc_once_init.
//
// Scope it to the statements that MOVED, never to the enclosing block. Clearing
// a whole block makes the printer relay out everything in it, which destroys
// formatting the author chose and the mutation did not touch: on
// hard_conc_once_init that collapsed a three-line `once.Do(func(){...})` to one
// line and dropped a blank line elsewhere in the function, turning a one-line
// scar into a three-line one. Measured — 52 source lines in, 49 out for the block
// form versus 52 for this one.
//
// The set of fields below is the layout-relevant closure over the node types
// these operators can move — statements, and the expressions they contain. It is
// deliberately not exhaustive over go/ast: an unhandled node keeps its position,
// which degrades to the old blank-line artifact rather than producing wrong code.
// If a future operator moves a construct not listed here, add it.
func clearPositions(n ast.Node) func() {
	var restore []func()
	zero := func(p *token.Pos) {
		was := *p
		restore = append(restore, func() { *p = was })
		*p = token.NoPos
	}
	ast.Inspect(n, func(x ast.Node) bool {
		switch t := x.(type) {
		case *ast.Ident:
			zero(&t.NamePos)
		case *ast.BasicLit:
			zero(&t.ValuePos)
		case *ast.CallExpr:
			zero(&t.Lparen)
			zero(&t.Rparen)
		case *ast.BinaryExpr:
			zero(&t.OpPos)
		case *ast.UnaryExpr:
			zero(&t.OpPos)
		case *ast.ParenExpr:
			zero(&t.Lparen)
			zero(&t.Rparen)
		case *ast.IndexExpr:
			zero(&t.Lbrack)
			zero(&t.Rbrack)
		case *ast.CompositeLit:
			zero(&t.Lbrace)
			zero(&t.Rbrace)
		case *ast.AssignStmt:
			zero(&t.TokPos)
		case *ast.IncDecStmt:
			zero(&t.TokPos)
		case *ast.IfStmt:
			zero(&t.If)
		case *ast.ForStmt:
			zero(&t.For)
		case *ast.RangeStmt:
			zero(&t.For)
			zero(&t.TokPos)
		case *ast.BlockStmt:
			zero(&t.Lbrace)
			zero(&t.Rbrace)
		case *ast.ReturnStmt:
			zero(&t.Return)
		case *ast.DeferStmt:
			zero(&t.Defer)
		case *ast.GoStmt:
			zero(&t.Go)
		}
		return true
	})
	return func() {
		for i := len(restore) - 1; i >= 0; i-- {
			restore[i]()
		}
	}
}

// exitsIn reports whether any statement in stmts can leave the enclosing
// function or loop iteration without falling through: return, break, continue,
// goto, or a panic call. It is the veto for undeferring an Unlock — see the
// deferred-escape operator, where a `return` inside the covered region turns a
// would-be race into a deadlock.
//
// Nested function literals are NOT descended into, and that is the point: a
// `return` inside a `func() {...}` returns from the literal, so it does not skip
// the enclosing function's unlock. Descending would veto correct sites (a
// `once.Do(func(){ return })` is harmless here) and the operators are already
// starved for reach.
//
// It errs toward vetoing: `panic` is matched by callee name alone, so a local
// function called `panic` would be treated as an exit. That direction is right —
// a missed site costs one candidate, while a wrong site costs a mutant whose
// defect is not the one its description claims.
func exitsIn(stmts []ast.Stmt) bool {
	found := false
	for _, s := range stmts {
		ast.Inspect(s, func(n ast.Node) bool {
			if found {
				return false
			}
			switch t := n.(type) {
			case *ast.FuncLit:
				return false // its returns exit the literal, not us
			case *ast.ReturnStmt:
				found = true
			case *ast.BranchStmt:
				found = true // break, continue, goto, fallthrough
			case *ast.CallExpr:
				if id, ok := t.Fun.(*ast.Ident); ok && id.Name == "panic" {
					found = true
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// hasCommentIn reports whether any comment sits inside [from,to). The scar-free
// reordering operator uses this as a veto: moving a statement past a comment
// leaves the comment attached to the wrong line, and a stray or contradictory
// comment is itself a structural tell — exactly the scar these operators exist to
// avoid. Skipping such a site costs one candidate and protects the measurement.
func hasCommentIn(f *ast.File, from, to token.Pos) bool {
	for _, cg := range f.Comments {
		if cg.End() > from && cg.Pos() < to {
			return true
		}
	}
	return false
}

// collectScarFreeRaceSites enumerates race-introducing edits that leave the
// synchronization scaffolding *intact and balanced*, so the mutant reads as
// self-consistent code that happens to be wrong.
//
// This is the operator set collectRaceSites cannot provide. Sync-deletion always
// leaves a scar (a WaitGroup with Add/Done and no Wait; a Lock with no Unlock),
// and results/race-seeded-2026-07-25.md showed the judge scoring 20/20 on those
// by spotting the imbalance rather than by reasoning about interleaving — while
// false-accepting a *model-authored* race live. The reading-invisible class needs
// mutants with nothing missing. Three operators:
//
//  1. Lock/Unlock -> RLock/RUnlock, co-mutating the receiver's declaration from
//     sync.Mutex to sync.RWMutex. Every call is still present and paired; the
//     guarded write is simply no longer exclusive, and a read lock held over a
//     write is the reading-invisible defect par excellence — nothing is missing
//     from the page, the lock is simply the wrong one.
//
//     The co-mutation is not optional and its absence was a measurement bug. An
//     earlier version edited only the call sites and left the declaration alone,
//     relying on "the build filter enforces the RWMutex rather than a type
//     check". On a benchmark of plain sync.Mutex that silently yields ZERO
//     mutants — all six sites compile-fail on `mu.RLock undefined` — and
//     experiment 28 first reported that as a property of the corpus ("no
//     sync.RWMutex anywhere in examples/bench") rather than of the operator.
//     Deferring a type constraint to the compiler turns an unreachable operator
//     into an empty result, and an empty result reads as a finding. Both halves
//     of the type change belong to the same edit.
//
//  2. Move the last guarded statement out of the critical section, past the
//     Unlock. The Lock/Unlock pair survives untouched and the section stays
//     non-empty, so nothing reads as dangling.
//
//     2b. The same escape against a DEFERRED unlock: `Lock(); defer Unlock(); A; B`
//     becomes `Lock(); A; Unlock(); B`. This needs the extra step of converting
//     the defer to a plain call — there is no way to shorten a defer's scope
//     in place — which is why it was skipped by the first version of operator 2
//     and why it needs its own case. It matters because `Lock(); defer Unlock()`
//     is, in syncCall's own words, the dominant Go idiom: on the concurrency
//     benchmark the plain form appears in 3 references and the deferred form in
//     hard_conc_rate_limiter alone accounts for 3 further critical sections that
//     operator 2 could not touch.
//
//     The mutant is scar-free on a subtler axis than the others: `Lock(); A;
//     Unlock(); B` is an entirely idiomatic shape. Nothing is missing, nothing is
//     unbalanced, and the defer is gone rather than dangling — a reader has to
//     notice that B touches guarded state.
//
//  3. `wg.Wait()` -> `defer wg.Wait()`. The Wait is still there and still runs;
//     it just no longer happens before the statements that read what the
//     goroutines wrote.
//
// Deliberately NOT included: swapping a goroutine's parameter capture for a
// loop-variable capture, which results/race-seeded-2026-07-25.md:58-61 proposed.
// Go 1.22 made loop variables per-iteration, so on this toolchain that mutant
// does not race at all — verified, it passes `-race -count=3`. It would have been
// silently dropped by the -race filter and produced no seeds.
func collectScarFreeRaceSites(src string) ([]mutation, *token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}
	var sites []mutation
	ast.Inspect(f, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			recv, method, sel, ok := syncCall(fset, stmt)
			if !ok {
				continue
			}
			pos := fset.Position(stmt.Pos()).String()

			switch method {
			case "Lock":
				// Find the matching Unlock on the same receiver later in this block.
				unlockAt, unlockSel, deferred := -1, (*ast.SelectorExpr)(nil), false
				for j := i + 1; j < len(block.List); j++ {
					r2, m2, s2, ok2 := syncCall(fset, block.List[j])
					if !ok2 || r2 != recv || m2 != "Unlock" {
						continue
					}
					_, deferred = block.List[j].(*ast.DeferStmt)
					unlockAt, unlockSel = j, s2
					break
				}
				if unlockSel == nil {
					continue
				}

				// Operator 1: downgrade the pair to a read lock. All three edits
				// are one mutation — a lone RLock against an Unlock would not
				// compile, and worse, would be an imbalance a reader could spot,
				// while RLock against a plain sync.Mutex does not compile at all.
				// The declaration comes along; see the operator's doc comment for
				// why leaving it behind is a measurement bug and not a shortcut.
				lockSel, unlSel := sel, unlockSel
				if decl := mutexTypeOf(f, recv); decl != nil {
					// Capture the original type name rather than assuming "Mutex":
					// on a source already declaring an RWMutex, reverting to Mutex
					// would leave the AST wrong for every later site in the file.
					was := decl.Sel.Name
					sites = append(sites, mutation{
						apply: func() {
							lockSel.Sel.Name, unlSel.Sel.Name = "RLock", "RUnlock"
							decl.Sel.Name = "RWMutex"
						},
						revert: func() {
							lockSel.Sel.Name, unlSel.Sel.Name = "Lock", "Unlock"
							decl.Sel.Name = was
						},
						desc: pos + ": downgrade " + recv +
							" Lock/Unlock to RLock/RUnlock, declaration to sync.RWMutex" +
							" (guarded write loses exclusivity)",
					})
				}

				// Operator 2b: the same escape when the Unlock is DEFERRED. The
				// deferred call cannot have its scope shortened in place, so the
				// edit replaces it with a plain call positioned before the last
				// guarded statement:
				//
				//   Lock(); defer Unlock(); A; B   ->   Lock(); A; Unlock(); B
				//
				// Requires at least two statements after the defer so the critical
				// section keeps something in it, matching operator 2's guard.
				if deferred {
					guarded := block.List[unlockAt+1:]
					if len(guarded) < 2 {
						continue
					}
					// Everything from the defer to the block's end can move a line,
					// so the veto window spans that whole range rather than just the
					// two statements being reordered.
					if hasCommentIn(f, block.List[unlockAt].Pos(), block.Rbrace) {
						continue
					}
					// VETO: a control-flow exit anywhere in the region the defer was
					// covering. Undeferring only unlocks on the path that falls
					// through, so a `return` inside that region leaks the lock and the
					// mutant DEADLOCKS instead of racing.
					//
					// That is a different defect class wearing this operator's clothes,
					// and a worse one for this arm than a merely wrong answer on two
					// counts: a deadlock is refuted by a *timeout*, whose cause can be
					// external to the candidate (the one refutation with that property,
					// see StageResult.TimedOut), and a Lock with a `return` between it
					// and its Unlock is a visible imbalance — precisely the scar the
					// deletion operator already covers and this arm exists to avoid.
					// Measured on hard_conc_rate_limiter, where Allow's `return true`
					// inside the critical section produced exactly this mutant.
					if exitsIn(block.List[unlockAt+1:]) {
						continue
					}
					plain := &ast.ExprStmt{X: &ast.CallExpr{Fun: unlockSel}}
					orig := block.List
					mutated := make([]ast.Stmt, 0, len(orig))
					mutated = append(mutated, orig[:unlockAt]...)
					mutated = append(mutated, guarded[:len(guarded)-1]...)
					mutated = append(mutated, plain, guarded[len(guarded)-1])
					blk, escapee := block, fset.Position(guarded[len(guarded)-1].Pos()).String()
					var restore func()
					sites = append(sites, mutation{
						apply: func() {
							blk.List = mutated
							// Removing the defer shifts every following statement up
							// one slot, so the printer sees each of their recorded
							// lines jump and emits a blank line per jump (measured:
							// two on hard_conc_rate_limiter). Every statement from
							// the old defer onward genuinely moved, so clearing
							// exactly that range is still scoped to the edit — see
							// clearPositions on why the whole block is too wide.
							undos := make([]func(), 0, len(mutated)-unlockAt+1)
							for _, st := range mutated[unlockAt:] {
								undos = append(undos, clearPositions(st))
							}
							// The block itself is one line shorter now, so its
							// closing brace also has a stale line and the printer
							// pads before it. Clearing Rbrace alone (not the whole
							// block) removes that last blank line.
							wasRbrace := blk.Rbrace
							blk.Rbrace = token.NoPos
							undos = append(undos, func() { blk.Rbrace = wasRbrace })
							restore = func() {
								for i := len(undos) - 1; i >= 0; i-- {
									undos[i]()
								}
							}
						},
						revert: func() {
							if restore != nil {
								restore()
								restore = nil
							}
							blk.List = orig
						},
						desc: escapee + ": move statement out of the " + recv +
							" critical section, undeferring the Unlock",
					})
					continue
				}

				// Operator 2: move the last guarded statement out past the Unlock.
				// Requires at least two guarded statements, so the critical section
				// does not end up empty — an empty one is itself a tell.
				if unlockAt-i < 3 {
					continue
				}
				a, b := unlockAt-1, unlockAt
				// The veto window starts at the END of the statement before `a`, not
				// at a.Pos(): a comment on its own line above `a` is a *leading*
				// comment, so it sits before a.Pos() and would escape a narrower
				// window. It ends at the start of the statement after `b` (or the
				// block's close) to catch a trailing comment on the Unlock line.
				// unlockAt-i >= 3 guarantees a-1 > i, so the Lock itself is excluded.
				to := block.Rbrace
				if b+1 < len(block.List) {
					to = block.List[b+1].Pos()
				}
				if hasCommentIn(f, block.List[a-1].End(), to) {
					continue
				}
				escapee := fset.Position(block.List[a].Pos()).String()
				blk := block
				var restore func()
				sites = append(sites, mutation{
					apply: func() {
						blk.List[a], blk.List[b] = blk.List[b], blk.List[a]
						// Same reason as operator 2b: swapping two statements makes
						// their recorded lines run backwards and go/printer emits a
						// blank line at the mutation site. Measured on
						// hard_conc_once_init, where this operator's one site was
						// the only site of fifteen to gain a blank line. Clear only
						// the two swapped statements, not the block.
						undo1 := clearPositions(blk.List[a])
						undo2 := clearPositions(blk.List[b])
						restore = func() { undo2(); undo1() }
					},
					revert: func() {
						if restore != nil {
							restore()
							restore = nil
						}
						blk.List[a], blk.List[b] = blk.List[b], blk.List[a]
					},
					desc: escapee + ": move statement out of the " + recv + " critical section",
				})

			case "Wait":
				// Operator 3: defer the wait. Pointless unless something after it
				// reads what the goroutines write — with nothing following, the
				// deferred Wait still happens before the caller observes anything,
				// so the mutant would not race and the filter would discard it
				// after paying for a build and a -race run.
				if _, alreadyDeferred := stmt.(*ast.DeferStmt); alreadyDeferred {
					continue
				}
				if i == len(block.List)-1 {
					continue
				}
				exprStmt, ok := stmt.(*ast.ExprStmt)
				if !ok {
					continue
				}
				call, ok := exprStmt.X.(*ast.CallExpr)
				if !ok {
					continue
				}
				idx, orig := i, stmt
				deferStmt := &ast.DeferStmt{Defer: exprStmt.Pos(), Call: call}
				sites = append(sites, mutation{
					apply:  func() { block.List[idx] = deferStmt },
					revert: func() { block.List[idx] = orig },
					desc:   pos + ": defer " + recv + ".Wait() (it runs, but no longer before the reads)",
				})
			}
		}
		return true
	})
	return sites, fset, f, nil
}

// RaceKilledMutants returns up to n mutant sources that COMPILE and are refuted
// by the race detector (`go test -race`). Each deletes one synchronization
// statement, producing a genuine data race — the seed set for probing the
// judge's one observed blind spot (paper §3.1; the sole live η_fa in the study
// was a race). Whether the mutant also flakes without -race is not a filter:
// flakiness is precisely what a reader cannot rely on.
//
// CAVEAT (see results/race-seeded-2026-07-25.md): deletion leaves a visible
// scar — a WaitGroup with Add/Done but no Wait, or a Lock with no Unlock — which
// a competent reviewer catches by imbalance, not by reasoning about the race. So
// these mutants test races-with-a-structural-tell, NOT the scar-free,
// self-consistent racy code that was actually false-accepted live.
// ScarFreeRaceKilledMutants covers that class. Both are kept: the *comparison*
// between the two η_fa rates is the result, and a single pooled rate would
// average away the distinction the experiment exists to draw.
func RaceKilledMutants(ctx context.Context, r *Runner, src, visible, hidden string, n, raceCount int, timeout time.Duration) ([]KilledMutant, error) {
	sites, fset, f, err := collectRaceSites(src)
	if err != nil {
		return nil, err
	}
	return raceKilledFrom(ctx, r, sites, fset, f, src, visible, hidden, n, raceCount, timeout)
}

// ScarFreeRaceKilledMutants is RaceKilledMutants over the scar-free operator set:
// mutants whose synchronization scaffolding is intact and balanced, so a reader
// finds nothing missing. This is the seed generator for the class §3.1 is
// actually about, and the one the deletion operator provably cannot reach.
//
// It can legitimately return nothing. The operators need a mutex-guarded section,
// a critical section with two or more statements, or a Wait with reads after it; a
// solution with none of those yields no sites. That is a property of the program,
// not a failure, and the caller must report it as coverage rather than as η_fa = 0.
//
// Unlike the deletion arm this one requires an actual ThreadSanitizer report, not
// merely a `-race` failure. The whole point of the arm is that the defect is a
// race a reader cannot see; a mutant that returns the wrong answer deterministically
// is a different defect class wearing this operator's clothes, and counting it
// inflates the denominator with seeds that test nothing about race-blindness.
// Experiment 28's first pass did exactly that on conc_safe_counter.
func ScarFreeRaceKilledMutants(ctx context.Context, r *Runner, src, visible, hidden string, n, raceCount int, timeout time.Duration) ([]KilledMutant, error) {
	sites, fset, f, err := collectScarFreeRaceSites(src)
	if err != nil {
		return nil, err
	}
	all, err := raceKilledFrom(ctx, r, sites, fset, f, src, visible, hidden, n, raceCount, timeout)
	if err != nil {
		return nil, err
	}
	out := make([]KilledMutant, 0, len(all))
	for _, m := range all {
		if m.DataRace {
			out = append(out, m)
		}
	}
	return out, nil
}

// raceKilledFrom is the harvest loop shared by both race operator sets: print
// each mutant, keep it only if it compiles and the race detector refutes it.
// Shared on purpose — if the two sets were filtered differently, their η_fa rates
// would not be comparable and comparing them is the entire point.
func raceKilledFrom(ctx context.Context, r *Runner, sites []mutation, fset *token.FileSet, f *ast.File,
	src, visible, hidden string, n, raceCount int, timeout time.Duration,
) ([]KilledMutant, error) {
	if len(sites) == 0 || n <= 0 {
		return nil, nil
	}
	if raceCount < 1 {
		raceCount = 1
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
			continue // does not compile
		}
		// The defining property of a race-class defect is that the race detector
		// refutes it. We require a -race failure and do NOT require the plain run
		// to pass: a synchronization deletion may or may not flake without -race
		// depending on timing, but the reader-invisibility is intrinsic to the
		// defect (a missing lock reads as fine on the page) regardless. Requiring
		// plain to pass would discard genuine races that happen to also flake, and
		// flakiness is exactly what a reader cannot rely on catching.
		race := ws.run(ctx, timeout*time.Duration(raceCount)+30*time.Second, true,
			"test", "-race", "-count="+strconv.Itoa(raceCount), "./...")
		if race.Err == nil && !race.TimedOut {
			continue
		}
		// Distinguish "ThreadSanitizer reported a race" from "failed under -race",
		// and record whether the plain run refutes it too. Neither is a filter here
		// — the shared harvest must treat both arms identically or their η_fa rates
		// stop being comparable, which is the entire reason this loop is shared.
		// ScarFreeRaceKilledMutants applies the DataRace requirement on top.
		plain := ws.run(ctx, timeout*time.Duration(raceCount)+30*time.Second, false,
			"test", "-count="+strconv.Itoa(raceCount), "./...")
		out = append(out, KilledMutant{
			Source:       mutant,
			Desc:         m.desc,
			DataRace:     strings.Contains(race.Output, "DATA RACE"),
			PlainRefuted: plain.Err != nil || plain.TimedOut,
		})
	}
	return out, nil
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
