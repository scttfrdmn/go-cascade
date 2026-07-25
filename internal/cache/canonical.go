package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// CanonicalHash reduces a solution to a hash that is stable under comment,
// formatting and local-identifier changes.
//
// The alpha-renaming here is scope-approximate: it collapses shadowed names.
// That is deliberate. A cache key only has to be a good retrieval heuristic,
// because a retrieved entry is never trusted — it is re-executed against the
// new query's tests before it can be returned. A key collision therefore costs
// one wasted verification, not a wrong answer.
func CanonicalHash(src string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	f.Doc = nil
	f.Comments = nil

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		fd.Doc = nil
		renameLocals(fd)
	}

	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.RawFormat, Tabwidth: 1}
	if err := cfg.Fprint(&buf, fset, f); err != nil {
		return "", err
	}
	norm := strings.Join(strings.Fields(buf.String()), " ")
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:16]), nil
}

// renameLocals rewrites parameters, results and locally declared variables to
// positional names in declaration order.
func renameLocals(fd *ast.FuncDecl) {
	names := map[string]string{}
	n := 0
	declare := func(id *ast.Ident) {
		if id == nil || id.Name == "_" || id.Name == "" {
			return
		}
		if _, seen := names[id.Name]; seen {
			return
		}
		names[id.Name] = "v" + itoa(n)
		n++
	}

	if fd.Recv != nil {
		for _, fld := range fd.Recv.List {
			for _, id := range fld.Names {
				declare(id)
			}
		}
	}
	if fd.Type.Params != nil {
		for _, fld := range fd.Type.Params.List {
			for _, id := range fld.Names {
				declare(id)
			}
		}
	}
	if fd.Type.Results != nil {
		for _, fld := range fd.Type.Results.List {
			for _, id := range fld.Names {
				declare(id)
			}
		}
	}
	ast.Inspect(fd.Body, func(nd ast.Node) bool {
		switch t := nd.(type) {
		case *ast.AssignStmt:
			if t.Tok == token.DEFINE {
				for _, lhs := range t.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						declare(id)
					}
				}
			}
		case *ast.ValueSpec:
			for _, id := range t.Names {
				declare(id)
			}
		case *ast.RangeStmt:
			if id, ok := t.Key.(*ast.Ident); ok {
				declare(id)
			}
			if id, ok := t.Value.(*ast.Ident); ok {
				declare(id)
			}
		}
		return true
	})

	ast.Inspect(fd, func(nd ast.Node) bool {
		// Selector fields are not locals: rewriting x.Foo would corrupt the
		// meaning, so only the receiver expression is visited.
		if sel, ok := nd.(*ast.SelectorExpr); ok {
			ast.Inspect(sel.X, func(inner ast.Node) bool {
				if id, ok := inner.(*ast.Ident); ok {
					if to, ok := names[id.Name]; ok {
						id.Name = to
					}
				}
				return true
			})
			return false
		}
		if id, ok := nd.(*ast.Ident); ok {
			if to, ok := names[id.Name]; ok {
				id.Name = to
			}
		}
		return true
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// NormalizeProblem lowercases and collapses whitespace so trivially different
// phrasings of the same request share a key.
func NormalizeProblem(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ProblemHash keys the test cache.
func ProblemHash(s string) string {
	sum := sha256.Sum256([]byte(NormalizeProblem(s)))
	return hex.EncodeToString(sum[:16])
}

// trigrams builds the character-trigram set used for retrieval.
func trigrams(s string) map[string]struct{} {
	s = NormalizeProblem(s)
	out := map[string]struct{}{}
	r := []rune(s)
	for i := 0; i+3 <= len(r); i++ {
		out[string(r[i:i+3])] = struct{}{}
	}
	return out
}

// Similarity is Jaccard overlap on character trigrams.
//
// This is deliberately not an embedding: retrieval quality only affects how
// often the cache is worth consulting, never whether its answer is correct, so
// a dependency-free lexical measure is the right trade. Swap in an embedding
// model if recall on paraphrases matters more than the extra call.
func Similarity(a, b string) float64 {
	ta, tb := trigrams(a), trigrams(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for t := range ta {
		if _, ok := tb[t]; ok {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
