// Package readguard holds a source check, no runtime code.
package readguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// allowed lists the files that may call os.ReadFile directly, with why.
var allowed = map[string]string{
	"account/switchlog.go": "append-only log, never renamed over",
	"wrapper/request.go":   "claimed copy is private to the reader",
	"fsutil/readfile.go":   "the retrying helper itself",
}

// TestNoPlainReadFile fails when a non-test file under internal/ calls
// os.ReadFile: files clalt or claude replace by rename must be read with
// fsutil.ReadFile so Windows sharing violations are retried.
func TestNoPlainReadFile(t *testing.T) {
	root := ".."
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "ReadFile" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "os" {
				if _, ok := allowed[rel]; !ok {
					t.Errorf("%s: os.ReadFile used, use fsutil.ReadFile", fset.Position(sel.Pos()))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
