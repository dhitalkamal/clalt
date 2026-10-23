package live

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

func regSessions(f files, now time.Time) *RegistrySessions {
	return &RegistrySessions{Reg: f.reg, Feed: f.fd, Now: func() time.Time { return now }}
}

func TestRegistryAndFeedJoinIntoSessions(t *testing.T) {
	f := newFiles(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/feat/x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.record(t, h1, "work", epoch.Add(-time.Hour), func(r *registry.Record) { r.ClaudeSessionID = "c1" })
	pct := 50.0
	f.state(t, h1, "work", epoch, func(s *feed.SessionState) {
		s.State, s.Reason, s.Model, s.Cwd, s.ContextPct = feed.StateNeedsYou, "permission_prompt", "claude-opus-5-5", repo, &pct
	})
	got, err := regSessions(f, epoch).Running(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v err %v", got, err)
	}
	s := got[0]
	if s.Handle != h1 || s.AccountID != "work" || s.ClaudeSessionID != "c1" || s.State != StateNeedsYou || s.Reason != "permission_prompt" {
		t.Errorf("session = %+v", s)
	}
	if s.Model != "claude-opus-5-5" || s.Cwd != repo || s.Branch != "feat/x" || s.ContextPct == nil || *s.ContextPct != 50 {
		t.Errorf("session = %+v", s)
	}
	if !s.StartedAt.Equal(epoch.Add(-time.Hour)) || !s.UpdatedAt.Equal(epoch) {
		t.Errorf("times = %v %v", s.StartedAt, s.UpdatedAt)
	}
}

func TestASessionWithNoFeedFileIsStartingInItsRecordedFolder(t *testing.T) {
	f := newFiles(t)
	f.record(t, h1, "work", epoch, nil)
	got, _ := regSessions(f, epoch).Running(context.Background())
	if len(got) != 1 || got[0].State != StateStarting || got[0].Cwd != "/work/app" {
		t.Errorf("got %+v", got)
	}
}

func TestADeadWrapperIsNotRunningAndItsFileStays(t *testing.T) {
	f := newFiles(t)
	f.record(t, h1, "work", epoch, func(r *registry.Record) { r.WrapperPID = deadPID(t) })
	f.record(t, h2, "work", epoch.Add(time.Minute), nil)
	got, err := regSessions(f, epoch).Running(context.Background())
	if err != nil || len(got) != 1 || got[0].Handle != h2 {
		t.Fatalf("got %+v err %v", got, err)
	}
	// the dashboard only reads: cleaning up dead records is the commands' job
	if _, ok, _ := f.reg.Get(h1); !ok {
		t.Error("the record of a dead session was removed by a read")
	}
}

func TestAnEndedFeedStateIsReportedAsEnded(t *testing.T) {
	f := newFiles(t)
	f.record(t, h1, "work", epoch, nil)
	f.state(t, h1, "work", epoch, func(s *feed.SessionState) { s.State = feed.StateEnded })
	got, _ := regSessions(f, epoch).Running(context.Background())
	if len(got) != 1 || got[0].State != StateEnded {
		t.Errorf("got %+v", got)
	}
}

func TestNoRegistryFolderIsNoSessions(t *testing.T) {
	f := newFiles(t)
	got, err := regSessions(f, epoch).Running(context.Background())
	if err != nil || len(got) != 0 {
		t.Errorf("got %+v err %v", got, err)
	}
}

func TestBranchReadsRefsDetachedHeadsAndWorktrees(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := gitBranch(sub); got != "" {
		t.Errorf("no repo: %q", got)
	}
	git := filepath.Join(root, ".git")
	if err := os.MkdirAll(git, 0o700); err != nil {
		t.Fatal(err)
	}
	head := func(dir, text string) {
		if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	head(git, "ref: refs/heads/main\n")
	if got := gitBranch(sub); got != "main" {
		t.Errorf("ref: %q", got)
	}
	head(git, "0123456789abcdef0123456789abcdef01234567\n")
	if got := gitBranch(sub); got != "0123456" {
		t.Errorf("detached: %q", got)
	}
	wt, gd := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+gd+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	head(gd, "ref: refs/heads/fix/y\n")
	if got := gitBranch(wt); got != "fix/y" {
		t.Errorf("worktree: %q", got)
	}
}

func TestBranchIgnoresHeadsThatAreHugeOrLongOrPointNowhere(t *testing.T) {
	root := t.TempDir()
	git := filepath.Join(root, ".git")
	if err := os.MkdirAll(git, 0o700); err != nil {
		t.Fatal(err)
	}
	put := func(path, text string) {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put(filepath.Join(git, "HEAD"), "ref: refs/heads/"+strings.Repeat("a", 10<<20))
	if got := gitBranch(root); got != "" {
		t.Errorf("10 MB head gave %d characters", len(got))
	}
	put(filepath.Join(git, "HEAD"), "ref: refs/heads/"+strings.Repeat("b", 300)+"\n")
	if got := gitBranch(root); got != "" {
		t.Errorf("a 300 character branch was shown")
	}
	// a worktree file may point anywhere: no HEAD there, no branch
	wt, empty := t.TempDir(), t.TempDir()
	put(filepath.Join(wt, ".git"), "gitdir: "+empty+"\n")
	if got := gitBranch(wt); got != "" {
		t.Errorf("a gitdir with no HEAD gave %q", got)
	}
	put(filepath.Join(wt, ".git"), "gitdir: "+strings.Repeat("x", 10<<20))
	if got := gitBranch(wt); got != "" {
		t.Errorf("a huge .git file gave %q", got)
	}
}
