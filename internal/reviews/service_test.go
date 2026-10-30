package reviews

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const reply = `{"type":"result","session_id":"sess-9","result":"{\"summary\":\"two issues\",\"findings\":[{\"severity\":\"high\",\"file\":\"a.go\",\"line\":3,\"title\":\"nil deref\",\"detail\":\"d\"},{\"severity\":\"low\",\"file\":\"b.go\",\"line\":9,\"title\":\"name\",\"detail\":\"d\"}]}"}`

type rig struct {
	t   *testing.T
	fx  *fakeExec
	svc *Service
	dir string
}

// newRig builds a service over temp folders and a fake exec. two pull
// requests ask for review: #7 small, #8 over 1500 changed lines (reviewed like any other).
func newRig(t *testing.T) *rig {
	t.Helper()
	dir := t.TempDir()
	fx := &fakeExec{}
	fx.on("search prs", `[{"number":7,"title":"Fix it","url":"https://x/7","isDraft":false,"author":{"login":"ann"},"repository":{"nameWithOwner":"o/r"}},{"number":8,"title":"Huge","url":"https://x/8","isDraft":false,"author":{"login":"bo"},"repository":{"nameWithOwner":"o/r"}}]`, nil)
	fx.on("pr view 7 ", `{"headRefOid":"1111111aaaa","additions":40,"deletions":10,"changedFiles":2,"title":"Fix it","url":"https://x/7","author":{"login":"ann"}}`, nil)
	fx.on("pr view 8 ", `{"headRefOid":"2222222bbbb","additions":1500,"deletions":1,"changedFiles":40,"title":"Huge","url":"https://x/8","author":{"login":"bo"}}`, nil)
	fx.on("pr diff 7", "diff --git a/a.go b/a.go\n+x", nil)
	fx.on("pr diff 8", "diff --git a/b.go b/b.go\n+y", nil)
	fx.on("claude -p", reply, nil)
	r := &rig{t: t, fx: fx, dir: dir}
	r.svc = &Service{
		Dir: dir, CacheDir: filepath.Join(dir, "cache"), WorkDir: filepath.Join(dir, "work"),
		GH: GH{X: fx}, Git: Git{X: fx}, Claude: fx, ClaudeBin: "claude",
		EnvFor: func(d string) []string { return []string{"CLAUDE_CONFIG_DIR=" + d} },
		Accounts: func() []Candidate {
			return []Candidate{{ID: "work", ConfigDir: "/a/work", FiveHourUsedPct: pct(70)}, {ID: "home", ConfigDir: "/a/home", FiveHourUsedPct: pct(5)}}
		},
		Now: func() time.Time { return time.Unix(5000, 0).UTC() },
	}
	if err := r.svc.SaveSettings(Config{Enabled: true, Repos: []string{"o/r"}}); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *rig) claudeCalls() []call {
	var out []call
	for _, c := range r.fx.calls {
		if c.Name == "claude" {
			out = append(out, c)
		}
	}
	return out
}

func TestTickReviewsAHugePRLikeAnyOtherInsteadOfSkippingIt(t *testing.T) {
	r := newRig(t)
	if err := r.svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	list, _ := r.svc.List()
	if len(list) != 2 {
		t.Fatalf("list = %+v", list)
	}
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok || got.Summary != "two issues" || len(got.Findings) != 2 || got.Account != "home" || got.Transcript != "sess-9" || got.Author != "ann" {
		t.Fatalf("review = %+v", got)
	}
	big, ok, _ := r.svc.Get("o-r-8-2222222")
	if !ok || big.Skipped != "" || len(big.Findings) != 2 || big.Summary != "two issues" {
		t.Fatalf("huge pr should be reviewed, got %+v", big)
	}
	if n := len(r.claudeCalls()); n != 2 {
		t.Fatalf("claude ran %d times, want 2 (the huge one is reviewed too)", n)
	}
	if st := r.svc.Status(); st.LastChecked == nil || st.Running != nil {
		t.Fatalf("status = %+v", st)
	}
}

// onlySmallPR narrows the rig's search to #7 so a test can count calls for one review.
func onlySmallPR(r *rig) {
	r.fx.rules = append([]fakeRule{{has: "search prs", out: `[{"number":7,"title":"Fix it","url":"https://x/7","isDraft":false,"author":{"login":"ann"},"repository":{"nameWithOwner":"o/r"}}]`}}, r.fx.rules...)
}

func TestTickDedupesByHeadSha(t *testing.T) {
	r := newRig(t)
	onlySmallPR(r)
	_ = r.svc.Tick(context.Background())
	_ = r.svc.Tick(context.Background())
	if n := len(r.claudeCalls()); n != 1 {
		t.Fatalf("second poll reran: %d", n)
	}
	// a new head commit is a new review
	r.fx.rules = append([]fakeRule{{has: "pr view 7 ", out: `{"headRefOid":"3333333cccc","additions":4,"deletions":1,"changedFiles":1,"title":"Fix it","url":"u","author":{"login":"ann"}}`}}, r.fx.rules...)
	_ = r.svc.Tick(context.Background())
	if n := len(r.claudeCalls()); n != 2 {
		t.Fatalf("new sha should run again: %d", n)
	}
}

func TestTickClaudeArgvIsReadOnly(t *testing.T) {
	r := newRig(t)
	_ = r.svc.Tick(context.Background())
	c := r.claudeCalls()[0]
	line := strings.Join(c.Args, " ")
	for _, want := range []string{"-p", "--output-format json", "--tools Read,Grep,Glob", "--allowedTools Read,Grep,Glob", "--permission-mode dontAsk", "--setting-sources user", "--strict-mcp-config", "--session-id "} {
		if !strings.Contains(line, want) {
			t.Errorf("argv lacks %q: %s", want, line)
		}
	}
	if strings.Contains(strings.ReplaceAll(line, "--disallowedTools Bash,Edit,Write,NotebookEdit,WebFetch,WebSearch", ""), "Bash") {
		t.Errorf("Bash outside the deny list: %s", line)
	}
	if len(c.Env) != 1 || c.Env[0] != "CLAUDE_CONFIG_DIR=/a/home" {
		t.Errorf("env = %v (the account with the most room)", c.Env)
	}
	if !strings.Contains(c.Stdin, "diff --git a/a.go") || !strings.Contains(c.Stdin, "Fix it") {
		t.Errorf("prompt lacks the diff or title: %q", c.Stdin)
	}
	if !strings.HasPrefix(c.Dir, filepath.Join(r.dir, "work")) {
		t.Errorf("claude ran in %s, not in the temp worktree", c.Dir)
	}
	if _, err := os.Stat(c.Dir); err == nil {
		t.Errorf("worktree %s was not cleaned", c.Dir)
	}
}

func TestTickOnlyAllowlistedGHAndNoPostGit(t *testing.T) {
	r := newRig(t)
	_ = r.svc.Tick(context.Background())
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
		case "claude":
		default:
			t.Errorf("ran %s", c.Name)
		}
	}
}

func TestTickCloneOnceThenWorktree(t *testing.T) {
	r := newRig(t)
	onlySmallPR(r)
	_ = r.svc.Tick(context.Background())
	var clones, adds, removes int
	for _, l := range r.fx.lines() {
		switch {
		case strings.HasPrefix(l, "gh repo clone o/r "):
			clones++
		case strings.HasPrefix(l, "git worktree add --detach "):
			adds++
		case strings.HasPrefix(l, "git worktree remove --force "):
			removes++
		}
	}
	if clones != 1 || adds != 1 || removes != 1 {
		t.Fatalf("clone %d add %d remove %d: %v", clones, adds, removes, r.fx.lines())
	}
}

func TestTickOffOrUnlistedDoesNothing(t *testing.T) {
	r := newRig(t)
	_ = r.svc.SaveSettings(Config{Enabled: false, Repos: []string{"o/r"}})
	_ = r.svc.Tick(context.Background())
	_ = r.svc.SaveSettings(Config{Enabled: true, Repos: []string{}})
	_ = r.svc.Tick(context.Background())
	if len(r.fx.calls) != 0 {
		t.Fatalf("ran %v", r.fx.lines())
	}
}

func TestTickNoRoomStoresNothing(t *testing.T) {
	r := newRig(t)
	r.svc.Accounts = func() []Candidate { return []Candidate{{ID: "w", ConfigDir: "/a", FiveHourUsedPct: pct(97)}} }
	_ = r.svc.Tick(context.Background())
	list, _ := r.svc.List()
	if len(r.claudeCalls()) != 0 {
		t.Fatal("claude ran with no room")
	}
	for _, rv := range list {
		if rv.Number == 7 {
			t.Fatalf("stored %+v, it must be retried later", rv)
		}
	}
}

func TestTickClaudeFailureIsStoredWithError(t *testing.T) {
	r := newRig(t)
	r.fx.rules = append([]fakeRule{{has: "claude -p", err: errors.New("claude exited 1: limit reached")}}, r.fx.rules...)
	_ = r.svc.Tick(context.Background())
	got, ok, _ := r.svc.Get("o-r-7-1111111")
	if !ok || !strings.Contains(got.Error, "limit reached") {
		t.Fatalf("%+v", got)
	}
}

func TestStatusShowsRunningDuringReview(t *testing.T) {
	r := newRig(t)
	// one entry per claude call, in call order, so each review is checked on its own
	var seen []*Running
	r.fx.hook = func(c call) {
		if c.Name == "claude" {
			seen = append(seen, r.svc.Status().Running)
		}
	}
	_ = r.svc.Tick(context.Background())
	if len(seen) != 2 {
		t.Fatalf("claude calls = %d, want 2 (#7 and #8)", len(seen))
	}
	for i, want := range []int{7, 8} {
		if seen[i] == nil || seen[i].Repo != "o/r" || seen[i].Number != want {
			t.Fatalf("running during call %d = %+v, want o/r #%d", i, seen[i], want)
		}
	}
}

func TestUnparsableReplyKeepsTextAsSummary(t *testing.T) {
	r := newRig(t)
	r.fx.rules = append([]fakeRule{{has: "claude -p", out: `{"result":"looks fine to me","session_id":"s1"}`}}, r.fx.rules...)
	_ = r.svc.Tick(context.Background())
	got, _, _ := r.svc.Get("o-r-7-1111111")
	if got.Summary != "looks fine to me" || len(got.Findings) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestSettingsSavedAndSuggestions(t *testing.T) {
	r := newRig(t)
	r.fx.on("repo list", `[{"nameWithOwner":"o/r"},{"nameWithOwner":"o/z"}]`, nil)
	v := r.svc.SettingsView(context.Background())
	if !v.Enabled || len(v.Repos) != 1 || len(v.Suggestions) != 1 || v.Suggestions[0] != "o/z" {
		t.Fatalf("view = %+v (suggestions leave out the opted-in ones)", v)
	}
	c, err := LoadConfig(r.dir)
	if err != nil || !c.Enabled {
		t.Fatalf("stored = %+v %v", c, err)
	}
}
