package verify

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// Static holds facts computed from the AST alone. Every field here is a
// measurement, not a prediction: these gates are sound by construction and so
// consume none of the risk budget.
type Static struct {
	Imports         []string
	NonStdImports   []string
	UsesConcurrency bool // gates the race stage
	MaxComplexity   int  // max cyclomatic complexity over all functions
	ComplexFunc     string
	Funcs           int
}

// Analyse parses src and computes the static facts. A parse error is returned
// so the caller can report it as stage V0.
func Analyse(src string) (*Static, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "solution.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	s := &Static{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		s.Imports = append(s.Imports, path)
		if !isStdlib(path) {
			s.NonStdImports = append(s.NonStdImports, path)
		}
		if path == "sync" || path == "sync/atomic" || path == "context" ||
			strings.HasPrefix(path, "golang.org/x/sync") {
			s.UsesConcurrency = true
		}
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.GoStmt, *ast.SelectStmt, *ast.ChanType:
			s.UsesConcurrency = true
		}
		return true
	})

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		s.Funcs++
		if c := complexity(fd); c > s.MaxComplexity {
			s.MaxComplexity = c
			s.ComplexFunc = fd.Name.Name
		}
	}
	return s, nil
}

// complexity is McCabe cyclomatic complexity: one per function plus one per
// branch point.
func complexity(fd *ast.FuncDecl) int {
	c := 1
	ast.Inspect(fd, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CommClause:
			c++
		case *ast.CaseClause:
			if len(t.List) > 0 { // default: is not a branch point
				c++
			}
		case *ast.BinaryExpr:
			if t.Op == token.LAND || t.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}

// isStdlib reports whether an import path belongs to the standard library.
// The go command's own rule: a path whose first element contains a dot is a
// module path, everything else is stdlib.
func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
