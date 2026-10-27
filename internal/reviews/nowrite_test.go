package reviews

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// forbidden are words that would turn a gh or git argv into something that
// writes: a review, a comment, a merge, an api call, a send to the remote. no
// string literal of the package may be one of them.
var forbidden = map[string]bool{
	"review": true, "comment": true, "merge": true, "edit": true, "close": true, "reopen": true, "ready": true,
	"api": true, "create": true, "delete": true, "push": true, "commit": true, "lock": true, "unlock": true,
	"--approve": true, "--request-changes": true, "--comment": true, "--body": true, "--body-file": true,
	"POST": true, "PATCH": true, "PUT": true, "DELETE": true, "-X": true, "--method": true, "--field": true, "--raw-field": true,
}

// violations lists the string literals of one Go source that are write words
// or that start with gh or git and then name one.
func violations(fset *token.FileSet, f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		bad := forbidden[v]
		if strings.HasPrefix(v, "gh ") || strings.HasPrefix(v, "git ") {
			for _, w := range strings.Fields(v) {
				bad = bad || forbidden[w]
			}
		}
		if bad {
			out = append(out, fset.Position(lit.Pos()).String()+": "+v)
		}
		return true
	})
	return out
}

// TestCodeNeverBuildsWriteArgv reads every non-test file of the package and
// fails when a string literal is a write word, or when a literal joins gh or
// git with one (as in "gh pr review"). the argv of gh and git is built from
// literals only, so this is the net under the allowlist.
func TestCodeNeverBuildsWriteArgv(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no sources: %v", err)
	}
	fset := token.NewFileSet()
	n := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, raw, 0)
		if err != nil {
			t.Fatal(err)
		}
		n++
		for _, v := range violations(fset, f) {
			t.Errorf("write word in argv code: %s", v)
		}
	}
	if n == 0 {
		t.Fatal("scanned nothing")
	}
}

// TestScannerCatchesWrites proves the scanner itself works.
func TestScannerCatchesWrites(t *testing.T) {
	for _, src := range []string{
		`package x; var a = []string{"pr", "review", "5"}`,
		`package x; var a = "gh pr comment 5"`,
		`package x; var a = "POST"`,
		`package x; var a = "api"`,
		`package x; var a = "git remote push"`,
	} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(violations(fset, f)) == 0 {
			t.Errorf("missed: %s", src)
		}
	}
}
