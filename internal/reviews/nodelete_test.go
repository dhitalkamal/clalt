package reviews

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// reviewFiles lists the names of the review json files in dir, sorted.
// config.json is not a review id and is left out.
func reviewFiles(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if ok && idRe.MatchString(id) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// listedIDs is the sorted set of ids Store.List returns.
func listedIDs(t *testing.T, r *stateRig) []string {
	t.Helper()
	all, err := r.svc.store().List()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, rv := range all {
		out = append(out, rv.ID())
	}
	sort.Strings(out)
	return out
}

func TestRefreshStatesNeverDeletesAReview(t *testing.T) {
	r := newStateRig(t, true)
	old := time.Unix(100, 0).UTC()
	r.put("o/r", 1, "1111111", StateUnknown, nil)
	r.put("o/r", 2, "2222222", StateOpen, &old)
	r.put("o/r", 3, "3333333", StateMerged, &old)
	r.put("o/r", 4, "4444444", StateClosed, &old)
	// two commits of one pr
	r.put("o/r", 5, "5555555", StateOpen, &old)
	r.put("o/r", 5, "5555556", StateUnknown, nil)
	r.put("p/q", 1, "6666666", StateUnknown, nil)

	dir := r.svc.Dir
	wantFiles := reviewFiles(t, dir)
	wantIDs := listedIDs(t, r)
	if len(wantFiles) != 7 || len(wantIDs) != 7 {
		t.Fatalf("setup: files=%v ids=%v, want 7 each", wantFiles, wantIDs)
	}
	cfgPath := filepath.Join(dir, "config.json")
	cfgBefore, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	type answer struct {
		out string
		err error
	}
	rounds := []answer{
		{`{"state":"MERGED"}`, nil},
		{`{"state":"CLOSED"}`, nil},
		{"", errors.New("gh: network down")},
		{`not json at all`, nil},
		{`{"state":"DRAFTY"}`, nil},
		{`{"state":"OPEN"}`, nil},
		{`{"state":"MERGED"}`, nil},
	}
	for i, a := range rounds {
		// change what gh says for every pr, first matching rule wins
		r.fx.rules = nil
		for _, k := range []struct {
			repo string
			n    int
		}{{"o/r", 1}, {"o/r", 2}, {"o/r", 3}, {"o/r", 4}, {"o/r", 5}, {"p/q", 1}} {
			r.answer(k.repo, k.n, a.out, a.err)
		}
		r.svc.RefreshStates(context.Background())

		if got := reviewFiles(t, dir); !reflect.DeepEqual(got, wantFiles) {
			t.Fatalf("round %d: files = %v, want %v", i, got, wantFiles)
		}
		if got := listedIDs(t, r); !reflect.DeepEqual(got, wantIDs) {
			t.Fatalf("round %d: listed = %v, want %v", i, got, wantIDs)
		}
		cfgNow, err := os.ReadFile(cfgPath)
		if err != nil || string(cfgNow) != string(cfgBefore) {
			t.Fatalf("round %d: config.json changed or unreadable: err=%v", i, err)
		}
	}
}

// deleters are the package-qualified calls that remove files.
var deleters = map[string]bool{
	"os.Remove": true, "os.RemoveAll": true, "syscall.Unlink": true,
}

// deletions lists calls in one Go source to a function in deleters.
func deletions(fset *token.FileSet, f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if name := pkg.Name + "." + sel.Sel.Name; deleters[name] {
			out = append(out, fset.Position(call.Pos()).String()+": "+name)
		}
		return true
	})
	return out
}

// TestStoreAndRefreshCodeNeverDeleteFiles scans only store.go and state.go.
// runner.go removes a worktree folder on purpose and is out of scope.
func TestStoreAndRefreshCodeNeverDeleteFiles(t *testing.T) {
	fset := token.NewFileSet()
	for _, name := range []string{"store.go", "state.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("guarded file %s must exist: %v", name, err)
		}
		f, err := parser.ParseFile(fset, name, raw, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range deletions(fset, f) {
			t.Errorf("file deletion in review store code: %s", v)
		}
	}
}

// TestDeleteScannerCatchesRemoves proves the scanner itself works.
func TestDeleteScannerCatchesRemoves(t *testing.T) {
	for _, src := range []string{
		`package x; import "os"; func f(x string) { os.Remove(x) }`,
		`package x; import "os"; func f(x string) { _ = os.RemoveAll(x) }`,
		`package x; import "syscall"; func f(x string) { syscall.Unlink(x) }`,
	} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(deletions(fset, f)) == 0 {
			t.Errorf("missed: %s", src)
		}
	}
	// a harmless call is not flagged
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", `package x; import "os"; func f(x string) { os.ReadFile(x) }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := deletions(fset, f); len(n) != 0 {
		t.Errorf("false positive: %v", n)
	}
}
