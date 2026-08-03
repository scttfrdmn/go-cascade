package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"slices"
	"strings"
)

// Text-level agreement, for arm (e) of §5.5(2): self-consistency at matched cost.
//
// This is deliberately the WEAK signal §3.5 argues against, and it exists to be
// the foil. §3.5's claim is that for code, execution inverts the usual tradeoff:
// self-consistency votes on how candidates are *written*, behavioural clustering
// votes on what they *do*, and the latter dominates because running Go is cheap.
// Comparing the two needs both to be implemented on the same candidates, so the
// vote here must never consult a verifier — a text vote that peeked at execution
// would be behavioural clustering with extra steps, and the comparison would be
// rigged in §3.5's favour.
//
// The normalisation is as generous to self-consistency as is reasonable, for the
// same reason: a weak foil proves nothing. Formatting and comments are erased, so
// two candidates differing only in whitespace or commentary agree. What is *not*
// erased is identifier naming and statement structure, because erasing those
// would start approximating semantic equivalence, which is the thing execution is
// supposed to be measuring. Any residual pessimism therefore biases against arm
// (e), and that direction has to be stated when the result is read.

// TextKey is the self-consistency clustering key: normalised source, so
// candidates that differ only in formatting or comments vote together.
//
// Normalisation is best-effort by design. A candidate that does not parse cannot
// be canonicalised, so it falls back to its raw text with a distinguishing prefix
// — it still gets to vote, and still agrees with a byte-identical twin, but it
// cannot be silently pooled with the parseable ones. Returning a constant for all
// unparseable candidates would be worse: it would manufacture a spurious majority
// out of the failures, which is the mistake Behaviour avoids by keying refuted
// candidates on the stage that refuted them.
func TextKey(src string) string {
	norm, ok := normalizeGo(src)
	if !ok {
		// Whitespace-folded rather than raw, so trailing-newline noise does not split
		// two otherwise identical unparseable candidates.
		return "raw:" + hash(strings.Join(strings.Fields(src), " "))
	}
	return "src:" + hash(norm)
}

// normalizeGo reprints the file from its AST with comments dropped, which erases
// formatting and commentary while preserving names and structure.
func normalizeGo(src string) (string, bool) {
	fset := token.NewFileSet()
	// Parsed WITHOUT ParseComments so the printer has no comment map to reproduce:
	// two candidates whose only difference is how much they explain themselves are
	// the same vote. (Doc comments live on the AST nodes only when requested.)
	f, err := parser.ParseFile(fset, "cand.go", src, 0)
	if err != nil {
		return "", false
	}
	// Import order is not a behavioural difference and models emit it inconsistently.
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.IMPORT {
			continue
		}
		slices.SortFunc(g.Specs, func(a, b ast.Spec) int {
			return strings.Compare(specPath(a), specPath(b))
		})
	}
	var b strings.Builder
	if err := (&printer.Config{Mode: printer.RawFormat, Tabwidth: 8}).Fprint(&b, fset, f); err != nil {
		return "", false
	}
	// The printer reproduces blank lines from the original token positions, so
	// reprinting alone does not erase them — two draws differing only in vertical
	// whitespace would still split the vote. Fold them out, and trim per line, so
	// what remains is statement sequence and naming.
	var out strings.Builder
	for line := range strings.SplitSeq(b.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String(), true
}

func specPath(s ast.Spec) string {
	if is, ok := s.(*ast.ImportSpec); ok && is.Path != nil {
		return is.Path.Value
	}
	return ""
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// TextVote is the self-consistency decision: the plurality normalised-source
// class, with its mass and the index of a member.
//
// It reports raw mass, not a Wilson lower bound. That is not an oversight and not
// an inconsistency with invariant #9: the bound exists because cluster mass is
// used as a *routing score* compared against a calibrated threshold, and a raw
// proportion there lets a two-sample tier report 1.0 unconditionally (§4.3).
// Arm (e) does no routing and crosses no threshold — it votes once and is scored
// on the outcome — so mass is the quantity self-consistency actually specifies,
// and bounding it below would handicap the foil for no reason.
//
// Ties break toward the lowest candidate index, which is deterministic and
// independent of the sampling order.
func TextVote(cands []Candidate) (winner int, mass float64, ok bool) {
	if len(cands) == 0 {
		return 0, 0, false
	}
	type group struct {
		n   int
		rep int
	}
	byKey := map[string]*group{}
	order := make([]string, 0, len(cands))
	for _, c := range cands {
		k := TextKey(c.Source)
		g, seen := byKey[k]
		if !seen {
			g = &group{rep: c.Index}
			byKey[k] = g
			order = append(order, k)
		}
		g.n++
		if c.Index < g.rep {
			g.rep = c.Index
		}
	}

	best := ""
	for _, k := range order {
		g := byKey[k]
		switch {
		case best == "":
			best = k
		case g.n > byKey[best].n:
			best = k
		case g.n == byKey[best].n && g.rep < byKey[best].rep:
			best = k
		}
	}
	return byKey[best].rep, float64(byKey[best].n) / float64(len(cands)), true
}
