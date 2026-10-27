package reviews

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAllowedArgv(t *testing.T) {
	ok := [][]string{
		{"search", "prs", "--review-requested=@me", "--state=open", "--repo", "o/r", "--json", "number"},
		{"pr", "view", "5", "--repo", "o/r", "--json", "headRefOid"},
		{"pr", "diff", "5", "--repo", "o/r"},
		{"pr", "checkout", "5", "--repo", "o/r", "--detach"},
		{"repo", "list", "--json", "nameWithOwner"},
		{"repo", "clone", "o/r", "/tmp/x"},
	}
	for _, a := range ok {
		if err := allowedGH(a); err != nil {
			t.Errorf("%v refused: %v", a, err)
		}
	}
	bad := [][]string{
		nil,
		{"pr"},
		{"pr", "review", "5", "--approve"},
		{"pr", "comment", "5", "--body", "x"},
		{"pr", "merge", "5"},
		{"pr", "edit", "5"},
		{"pr", "close", "5"},
		{"pr", "ready", "5"},
		{"issue", "comment", "5"},
		{"api", "repos/o/r/pulls/5/reviews", "-X", "POST"},
		{"api", "graphql"},
		{"auth", "token"},
		{"auth", "status", "--show-token"},
		{"pr", "view", "5", "--body", "x"},
		{"pr", "diff", "5", "-F", "a=b"},
		{"repo", "delete", "o/r"},
		{"repo", "clone", "o/r", "--", "--upload-pack=x"},
	}
	for _, a := range bad {
		err := allowedGH(a)
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%v allowed: %v", a, err)
		}
	}
}

func TestGHRefusesBeforeRunning(t *testing.T) {
	fx := &fakeExec{}
	g := GH{X: fx}
	if _, err := g.run(context.Background(), "", "pr", "review", "5", "--approve"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("err = %v", err)
	}
	if len(fx.calls) != 0 {
		t.Fatalf("ran %v", fx.lines())
	}
}

func TestSearchRequested(t *testing.T) {
	fx := (&fakeExec{}).on("search prs", `[{"number":7,"title":"Fix it","url":"https://github.com/o/r/pull/7","isDraft":false,"author":{"login":"ann"},"repository":{"nameWithOwner":"o/r"}},{"number":8,"title":"WIP","url":"u","isDraft":true,"author":{"login":"bo"},"repository":{"nameWithOwner":"o/r"}}]`, nil)
	prs, err := GH{X: fx}.SearchRequested(context.Background(), "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 || prs[0].Number != 7 || prs[0].Author != "ann" || prs[0].Repo != "o/r" {
		t.Fatalf("prs = %+v (drafts are skipped)", prs)
	}
	got := fx.lines()[0]
	if !strings.HasPrefix(got, "gh search prs --review-requested=@me --state=open --repo o/r --json ") {
		t.Fatalf("argv = %s", got)
	}
}

func TestSearchRejectsBadRepo(t *testing.T) {
	if _, err := (GH{X: &fakeExec{}}).SearchRequested(context.Background(), "--web"); err == nil {
		t.Fatal("a flag as repo must be refused")
	}
}

func TestViewAndRepoList(t *testing.T) {
	fx := (&fakeExec{}).
		on("pr view", `{"headRefOid":"0123456789abcdef","additions":30,"deletions":12,"changedFiles":3,"title":"T","url":"U","author":{"login":"ann"}}`, nil).
		on("repo list", `[{"nameWithOwner":"o/a"},{"nameWithOwner":"o/b"}]`, nil)
	g := GH{X: fx}
	v, err := g.View(context.Background(), "o/r", 7)
	if err != nil || v.SHA != "0123456789abcdef" || v.Lines != 42 || v.Files != 3 {
		t.Fatalf("view = %+v, %v", v, err)
	}
	repos, err := g.RepoList(context.Background())
	if err != nil || len(repos) != 2 || repos[1] != "o/b" {
		t.Fatalf("repos = %v, %v", repos, err)
	}
}
