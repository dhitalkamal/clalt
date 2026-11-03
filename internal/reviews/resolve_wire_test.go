package reviews

import (
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"testing"
)

// homebrewOnly is a resolver where only /opt/homebrew/bin has the tool.
func homebrewOnly(tool string) *Resolver {
	r, _ := newResolver(map[string]fs.FileMode{"/opt/homebrew/bin/" + tool: 0o755})
	return &r
}

// failingResolver finds nothing anywhere.
func failingResolver() *Resolver {
	r, _ := newResolver(nil)
	return &r
}

func TestGHRunsTheResolvedPathWhenPathLacksGh(t *testing.T) {
	fx := &fakeExec{}
	g := GH{X: fx, R: homebrewOnly("gh")}
	if _, err := g.Diff(context.Background(), "o/r", 7); err != nil {
		t.Fatal(err)
	}
	if len(fx.calls) != 1 || fx.calls[0].Name != "/opt/homebrew/bin/gh" {
		t.Fatalf("calls = %+v", fx.calls)
	}
}

func TestGHWithAFailingResolveRunsNothing(t *testing.T) {
	fx := &fakeExec{}
	g := GH{X: fx, R: failingResolver()}
	_, err := g.Diff(context.Background(), "o/r", 7)
	if err == nil {
		t.Fatal("want an error")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("err = %v, want the resolve error", err)
	}
	if len(fx.calls) != 0 {
		t.Fatalf("calls = %+v, want none", fx.calls)
	}
}

func TestGHWithNilResolverStillRunsBareGh(t *testing.T) {
	fx := &fakeExec{}
	g := GH{X: fx}
	if _, err := g.Diff(context.Background(), "o/r", 7); err != nil {
		t.Fatal(err)
	}
	if len(fx.calls) != 1 || fx.calls[0].Name != "gh" {
		t.Fatalf("calls = %+v", fx.calls)
	}
}

// claudeNames is the Name of every call that ran claude (bare or resolved).
func claudeNames(fx *fakeExec) []string {
	var out []string
	for _, c := range fx.calls {
		if len(c.Args) > 0 && c.Args[0] == "-p" {
			out = append(out, c.Name)
		}
	}
	return out
}

func TestReviewServiceClaudeRunUsesTheResolvedClaudePath(t *testing.T) {
	r := newRig(t)
	onlySmallPR(r)
	r.svc.R = homebrewOnly("claude")
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	names := claudeNames(r.fx)
	if len(names) != 1 || names[0] != "/opt/homebrew/bin/claude" {
		t.Fatalf("claude names = %v", names)
	}
}

func TestReviewServiceWithAnAbsoluteClaudeBinRunsItAsGivenEvenWhenMissing(t *testing.T) {
	r := newRig(t)
	onlySmallPR(r)
	r.svc.R = failingResolver()
	r.svc.ClaudeBin = "/nowhere/claude"
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	names := claudeNames(r.fx)
	if len(names) != 1 || names[0] != "/nowhere/claude" {
		t.Fatalf("claude names = %v", names)
	}
}
