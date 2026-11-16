package reviews

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

const baseSHA = "0123456789abcdef0123456789abcdef01234567"

// errTooManyFiles is what gh says when GitHub will not build a pull request
// diff (copied from a real failed review).
var errTooManyFiles = errors.New("/opt/homebrew/bin/gh failed: exit status 1: could not find pull request diff: " +
	"HTTP 406: Sorry, the diff exceeded the maximum number of files (300). Consider using " +
	"'List pull requests files' API or locally cloning the repository instead. (https:")

const localDiff = "diff --git a/big.go b/big.go\n+z\n"

// refusedRig reviews only #7, whose gh pr diff fails with diffErr. the base
// lookup and the local diff answer like a real repo.
func refusedRig(t *testing.T, diffErr error) *rig {
	t.Helper()
	r := newRig(t)
	onlySmallPR(r)
	r.fx.rules = append([]fakeRule{
		{has: "pr diff 7", err: diffErr},
		{has: "--json baseRefOid", out: `{"baseRefOid":"` + baseSHA + `"}`},
		{has: "git diff-tree", out: localDiff},
	}, r.fx.rules...)
	return r
}

func onlyReview(t *testing.T, r *rig) Review {
	t.Helper()
	list, err := r.svc.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("reviews = %v, %v; want one", list, err)
	}
	return list[0]
}

// callsOf lists the runs of name whose first argument is sub.
func callsOf(r *rig, name, sub string) []call {
	var out []call
	for _, c := range r.fx.calls {
		if c.Name == name && len(c.Args) > 0 && c.Args[0] == sub {
			out = append(out, c)
		}
	}
	return out
}

func checkoutDir(t *testing.T, r *rig) string {
	t.Helper()
	for _, c := range callsOf(r, "gh", "pr") {
		if len(c.Args) > 1 && c.Args[1] == "checkout" {
			return c.Dir
		}
	}
	t.Fatalf("no checkout in %v", r.fx.lines())
	return ""
}

func TestTickBuildsTheDiffLocallyWhenGitHubRefusesIt(t *testing.T) {
	r := refusedRig(t, errTooManyFiles)
	_ = r.svc.Tick(context.Background())
	if got := onlyReview(t, r); got.Error != "" {
		t.Fatalf("review failed: %s", got.Error)
	}
	cc := r.claudeCalls()
	if len(cc) == 0 || !strings.Contains(cc[0].Stdin, "diff --git a/big.go") {
		t.Fatalf("claude did not get the local diff: %v", r.fx.lines())
	}
	wt := checkoutDir(t, r)
	fetch := callsOf(r, "git", "fetch")
	if len(fetch) != 1 || !slices.Equal(fetch[0].Args, []string{"fetch", "--no-tags", "origin", baseSHA}) {
		t.Fatalf("fetch = %v", fetch)
	}
	if fetch[0].Dir != wt {
		t.Errorf("fetch ran in %s, not the worktree %s", fetch[0].Dir, wt)
	}
	if !slices.Contains(fetch[0].Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("fetch may stop to ask for a password: env has no GIT_TERMINAL_PROMPT=0")
	}
	diff := callsOf(r, "git", "diff-tree")
	want := []string{"diff-tree", "-p", "-r", "-M", "--merge-base", baseSHA, "HEAD"}
	if len(diff) != 1 || !slices.Equal(diff[0].Args, want) || diff[0].Dir != wt {
		t.Fatalf("diff-tree = %v, want %v in %s", diff, want, wt)
	}
	for _, c := range r.fx.calls {
		switch c.Name {
		case "gh":
			if err := allowedGH(c.Args); err != nil {
				t.Errorf("gh %v: %v", c.Args, err)
			}
		case "git":
			if err := allowedGit(c.Args); err != nil {
				t.Errorf("git %v: %v", c.Args, err)
			}
		}
	}
}

func TestTickKeepsOtherDiffErrorsWithoutTheLocalDiff(t *testing.T) {
	r := refusedRig(t, errors.New("gh failed: exit status 1: HTTP 401: Bad credentials"))
	_ = r.svc.Tick(context.Background())
	if got := onlyReview(t, r); !strings.Contains(got.Error, "Bad credentials") {
		t.Errorf("error = %q, want the gh error", got.Error)
	}
	if n := len(callsOf(r, "git", "fetch")) + len(callsOf(r, "git", "diff-tree")); n != 0 {
		t.Errorf("built a local diff for a non-size error: %v", r.fx.lines())
	}
	for _, l := range r.fx.lines() {
		if strings.Contains(l, "baseRefOid") {
			t.Errorf("looked up the base for a non-size error: %s", l)
		}
	}
}

func TestTickSaysBothWhenTheLocalDiffFailsToo(t *testing.T) {
	r := refusedRig(t, errTooManyFiles)
	r.fx.rules = append([]fakeRule{{has: "git fetch", err: errors.New("git failed: exit status 128: fatal: could not read Username")}}, r.fx.rules...)
	_ = r.svc.Tick(context.Background())
	got := onlyReview(t, r)
	if !strings.Contains(got.Error, "too big") || !strings.Contains(got.Error, "could not read Username") {
		t.Errorf("error = %q, want the refusal and the fetch error", got.Error)
	}
	if len(r.claudeCalls()) != 0 {
		t.Errorf("claude ran without a diff")
	}
	if len(callsOf(r, "git", "diff-tree")) != 0 {
		t.Errorf("diffed after a failed fetch")
	}
}

func TestBaseSHAReadsTheBaseCommit(t *testing.T) {
	fx := (&fakeExec{}).on("--json baseRefOid", `{"baseRefOid":"`+baseSHA+`"}`, nil)
	got, err := GH{X: fx}.BaseSHA(context.Background(), "o/r", 7)
	if err != nil || got != baseSHA {
		t.Fatalf("got %q, %v", got, err)
	}
	if l := fx.lines(); len(l) != 1 || l[0] != "gh pr view 7 --repo o/r --json baseRefOid" {
		t.Errorf("ran %v", l)
	}
}

func TestBaseSHARefusesAnythingButAFullCommitID(t *testing.T) {
	for _, v := range []string{"", "abc123", "--upload-pack=x", strings.ToUpper(baseSHA), baseSHA + "0", "main"} {
		fx := (&fakeExec{}).on("--json baseRefOid", `{"baseRefOid":"`+v+`"}`, nil)
		if got, err := (GH{X: fx}).BaseSHA(context.Background(), "o/r", 7); err == nil {
			t.Errorf("%q accepted as %q", v, got)
		}
	}
	fx := (&fakeExec{}).on("--json baseRefOid", `not json`, nil)
	if _, err := (GH{X: fx}).BaseSHA(context.Background(), "o/r", 7); err == nil {
		t.Errorf("bad json accepted")
	}
}

func TestAllowedGitTakesOnlyTheExactLocalDiffArgv(t *testing.T) {
	ok := [][]string{
		{"fetch", "--no-tags", "origin", baseSHA},
		{"diff-tree", "-p", "-r", "-M", "--merge-base", baseSHA, "HEAD"},
		{"worktree", "add", "--detach", "/tmp/wt"},
		{"worktree", "remove", "--force", "/tmp/wt"},
		{"worktree", "prune"},
	}
	for _, a := range ok {
		if err := allowedGit(a); err != nil {
			t.Errorf("%v refused: %v", a, err)
		}
	}
	bad := [][]string{
		{"fetch", "origin", baseSHA},
		{"fetch", "--no-tags", "upstream", baseSHA},
		{"fetch", "--no-tags", "origin", "main"},
		{"fetch", "--no-tags", "origin", "--upload-pack=x"},
		{"fetch", "--no-tags", "origin", baseSHA, "--upload-pack=x"},
		{"fetch", "--no-tags", "origin", baseSHA[:12]},
		{"diff-tree", "-p", "-r", "-M", "--merge-base", baseSHA, "main"},
		{"diff-tree", "-p", "-r", "-M", "--merge-base", "--output=/tmp/x", "HEAD"},
		{"diff-tree", "-p", "-r", "-M", "--merge-base", baseSHA, "HEAD", "--ext-diff"},
		{"diff-tree", "--ext-diff", "-r", "-M", "--merge-base", baseSHA, "HEAD"},
		{"diff", baseSHA + "...HEAD"},
		{"push", "origin", "HEAD"},
		{"remote", "add", "x", "y"},
	}
	for _, a := range bad {
		if err := allowedGit(a); err == nil {
			t.Errorf("%v allowed", a)
		}
	}
}
