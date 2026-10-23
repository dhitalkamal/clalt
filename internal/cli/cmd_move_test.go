package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/pick"
	"github.com/dhitalkamal/clalt/internal/wrapper"
)

const theHandle = "3fa9c1d2e4b5a607"

type moveRegistry struct {
	dir  string
	recs []wrapper.Record
}

func (r *moveRegistry) List() ([]wrapper.Record, error)     { return r.recs, nil }
func (r *moveRegistry) Dir() string                         { return r.dir }
func (r *moveRegistry) Update(string, wrapper.Change) error { return nil }

type moveFeed map[string]wrapper.SessionInfo

func (f moveFeed) Session(h string) (wrapper.SessionInfo, bool) { s, ok := f[h]; return s, ok }
func (f moveFeed) Limits(string) (pick.Limits, bool)            { return pick.Limits{}, false }

// withSession makes one running session of the account with this label, in cwd,
// with a feed state of Idle, and puts the registry and the feed where the
// command looks for them.
func withSession(t *testing.T, e *testEnv, label, cwd string) *moveRegistry {
	t.Helper()
	reg := &moveRegistry{dir: t.TempDir(), recs: []wrapper.Record{{
		Handle: theHandle, Account: e.account(label).Name, Folder: e.folder(label), WrapperPID: 4321,
		ClaudeSessionID: "7c5dcf5d-1111-4222-8333-444455556666", Cwd: cwd,
	}}}
	feed := moveFeed{theHandle: {State: wrapper.Idle}}
	oldReg, oldFeed, oldOS := openRegistry, openFeed, moveSupported
	openRegistry = func() (wrapper.Registry, error) { return reg, nil }
	openFeed = func() wrapper.Feed { return feed }
	moveSupported = true
	t.Cleanup(func() { openRegistry, openFeed, moveSupported = oldReg, oldFeed, oldOS })
	return reg
}

func requestOf(t *testing.T, reg *moveRegistry) (wrapper.Request, bool) {
	t.Helper()
	r, ok, err := wrapper.ClaimRequest(reg.dir, theHandle)
	if err != nil {
		t.Fatal(err)
	}
	return r, ok
}

func twoAccounts(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	e.signIn("personal", "p@example.com")
	return e
}

func TestMoveAsksTheWrapperToMoveAtTheNextPause(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")

	code := e.run("move", theHandle, "personal")

	if code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	req, ok := requestOf(t, reg)
	if !ok || req.To != e.account("personal").Name {
		t.Errorf("request = %+v, %v, want one for the id of personal", req, ok)
	}
	out := e.out.String()
	for _, want := range []string{"3fa9c1d2", "work", "personal", "next pause"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output lacks %q:\n%s", want, out)
		}
	}
	if e.claude.sessionStarted() {
		t.Error("move started a claude")
	}
}

func TestMoveFindsTheSessionByHandlePidOrSessionIdPrefixAndTheAccountByLabel(t *testing.T) {
	for _, ref := range []string{theHandle, "4321", "7c5d", "7C5DCF5D-1111"} {
		e := twoAccounts(t)
		reg := withSession(t, e, "work", "/x")

		if code := e.run("move", ref, "PERSONAL"); code != 0 {
			t.Errorf("%s: exit %d: %s", ref, code, e.errOut.String())
			continue
		}
		if _, ok := requestOf(t, reg); !ok {
			t.Errorf("%s: no request", ref)
		}
	}
}

func TestMoveRefusesWhatCannotWorkAndWritesNothing(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv, reg *moveRegistry, feed moveFeed)
		args  []string
		want  string
	}{
		{"no such session", nil, []string{"move", "ffffffffffffffff", "personal"}, "no running session"},
		{"no such account", nil, []string{"move", theHandle, "nobody"}, `no account called "nobody"`},
		{"the account it is on", nil, []string{"move", theHandle, "work"}, "already running as work"},
		{"a removed account", func(e *testEnv, _ *moveRegistry, _ moveFeed) {
			if code := e.run("remove", "personal", "--force"); code != 0 {
				e.t.Fatalf("remove exited %d", code)
			}
			e.reset()
		}, []string{"move", theHandle, "personal"}, "was removed"},
		{"folder rules of the target", func(e *testEnv, _ *moveRegistry, _ moveFeed) {
			e.setRules("personal", account.Rules{Folders: []string{subfolder(e.t, "elsewhere")}})
		}, []string{"move", theHandle, "personal"}, "folder rules"},
		{"a target that is not signed in", func(e *testEnv, _ *moveRegistry, _ moveFeed) {
			e.claude.signOut(e.folder("personal"))
		}, []string{"move", theHandle, "personal"}, "clalt login personal"},
		{"a session the feed knows nothing about", func(_ *testEnv, _ *moveRegistry, feed moveFeed) {
			delete(feed, theHandle)
		}, []string{"move", theHandle, "personal"}, "feed"},
		{"a session that has ended", func(_ *testEnv, _ *moveRegistry, feed moveFeed) {
			feed[theHandle] = wrapper.SessionInfo{State: wrapper.Ended}
		}, []string{"move", theHandle, "personal"}, "ended"},
		{"a session with no claude session id yet", func(_ *testEnv, reg *moveRegistry, _ moveFeed) {
			reg.recs[0].ClaudeSessionID = ""
		}, []string{"move", theHandle, "personal"}, "session id"},
		{"windows", func(*testEnv, *moveRegistry, moveFeed) { moveSupported = false }, []string{"move", theHandle, "personal"}, "not supported"},
	}
	for _, c := range cases {
		e := twoAccounts(t)
		reg := withSession(t, e, "work", "/x")
		if c.setup != nil {
			c.setup(e, reg, openFeed().(moveFeed))
		}

		code := e.run(c.args...)

		if code != 1 || !strings.Contains(e.errOut.String(), c.want) {
			t.Errorf("%s: exit %d, stderr %q, want 1 and %q", c.name, code, e.errOut.String(), c.want)
		}
		if _, ok := requestOf(t, reg); ok {
			t.Errorf("%s: a request was written", c.name)
		}
	}
}

func TestMoveChecksTheRulesAgainstTheFolderTheSessionIsInNow(t *testing.T) {
	e := twoAccounts(t)
	allowed, other := subfolder(t, "allowed"), subfolder(t, "other")
	e.setRules("personal", account.Rules{Folders: []string{allowed}})
	reg := withSession(t, e, "work", allowed)
	openFeed().(moveFeed)[theHandle] = wrapper.SessionInfo{State: wrapper.Idle, Cwd: other}

	code := e.run("move", theHandle, "personal")

	if code != 1 || !strings.Contains(e.errOut.String(), other) {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if _, ok := requestOf(t, reg); ok {
		t.Error("a request was written across a folder rule")
	}
}

func TestMoveAllowsATargetWhoseRulesCoverTheFolder(t *testing.T) {
	e := twoAccounts(t)
	dir := subfolder(t, "work")
	e.setRules("personal", account.Rules{Folders: []string{dir}})
	reg := withSession(t, e, "work", filepath.Join(dir, "app"))

	if code := e.run("move", theHandle, "personal"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}
	if _, ok := requestOf(t, reg); !ok {
		t.Error("no request")
	}
}

func TestASecondMoveRequestReplacesTheFirstAndSaysSo(t *testing.T) {
	e := twoAccounts(t)
	e.signIn("third", "t@example.com")
	reg := withSession(t, e, "work", "/x")
	e.run("move", theHandle, "personal")
	e.reset()

	if code := e.run("move", theHandle, "third"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if req, _ := requestOf(t, reg); req.To != e.account("third").Name {
		t.Errorf("request = %+v", req)
	}
	if !strings.Contains(e.out.String(), "replaced") {
		t.Errorf("the output does not say that the earlier request was replaced:\n%s", e.out.String())
	}
}

func TestMoveJSON(t *testing.T) {
	e := twoAccounts(t)
	withSession(t, e, "work", "/x")

	if code := e.run("move", theHandle, "personal", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var got map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatalf("not json: %v\n%s", err, e.out.String())
	}
	want := map[string]any{"handle": theHandle, "from": "work", "to": "personal", "requested": true, "replaced": false}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("json = %v, want %v", got, want)
	}
}

func TestMoveMistakesAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"move"}, {"move", theHandle}, {"move", theHandle, "personal", "extra"}, {"move", "--nope", theHandle, "personal"}} {
		e := twoAccounts(t)
		reg := withSession(t, e, "work", "/x")

		code := e.run(args...)

		if code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt move <session> <account>") {
			t.Errorf("%v: exit %d, stderr %q", args, code, e.errOut.String())
		}
		if _, ok := requestOf(t, reg); ok {
			t.Errorf("%v wrote a request", args)
		}
	}
}

// with no session running the real registry has nothing to match
func TestMoveWithNoRunningSessionSaysNoSessionMatches(t *testing.T) {
	e := twoAccounts(t)

	code := e.run("move", theHandle, "personal")

	if code != 1 || !strings.Contains(e.errOut.String(), "no running session matches") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

func TestMoveLeavesNoFileOutsideTheRegistry(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")
	e.run("move", theHandle, "personal")

	entries, _ := os.ReadDir(reg.dir)
	if len(entries) != 1 || entries[0].Name() != theHandle+".move" {
		t.Errorf("the registry folder holds %v", entries)
	}
}

func TestHelpListsTheMoveCommand(t *testing.T) {
	e := newEnv(t)
	e.run("help")
	for _, want := range []string{"clalt move <session> <account>", "next pause"} {
		if !strings.Contains(e.out.String(), want) {
			t.Errorf("the help is missing %q:\n%s", want, e.out.String())
		}
	}
}

func TestMoveSaysWhyTheEarlierRequestFailed(t *testing.T) {
	e := twoAccounts(t)
	reg := withSession(t, e, "work", "/x")
	if err := wrapper.WriteResult(reg.dir, theHandle, wrapper.Result{To: "personal", Error: "personal is not signed in"}); err != nil {
		t.Fatal(err)
	}

	if code := e.run("move", theHandle, "personal"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	if msg := e.errOut.String(); !strings.Contains(msg, "earlier request") || !strings.Contains(msg, "personal is not signed in") {
		t.Errorf("stderr:\n%s", msg)
	}
}
