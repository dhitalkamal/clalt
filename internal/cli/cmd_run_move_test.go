//go:build !windows

package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// waitUntil polls until ok says yes, or fails the test with what clalt said.
func (r *runningClalt) waitUntil(what string, ok func() bool) {
	r.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			r.t.Fatalf("gave up waiting for %s. clalt said: %s", what, r.said())
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (r *runningClalt) argv() string {
	raw, _ := os.ReadFile(r.file("session.argv"))
	return string(raw)
}

// the session that clalt run keeps is moved at its pause: a request from
// clalt move, a Stop hook that leaves the feed idle, and claude comes back with
// --resume under the other account, in the same clalt process.
func TestRunMovesTheSessionAtItsPauseWhenAskedWithClaltMove(t *testing.T) {
	r := startRunWith(t, false)
	e := r.e
	e.signIn("other", "o@example.com")
	other := e.account("other")

	recs, _, err := registry.Registry{Dir: config.SessionsDir()}.Scan()
	if err != nil || len(recs) != 1 {
		t.Fatalf("records: %v, %v", recs, err)
	}
	handle := recs[0].Handle
	reg := registry.Registry{Dir: config.SessionsDir()}
	store := feed.Store{Dir: config.FeedDir()}

	// what the SessionStart hook does: the session id, and the pause
	if _, _, err := reg.Update(handle, func(rec *registry.Record) { rec.ClaudeSessionID = "claude-sess-1" }); err != nil {
		t.Fatal(err)
	}
	err = store.UpdateSession(handle, recs[0].Account, time.Now(), func(s *feed.SessionState) bool {
		s.State, s.Since = feed.StateIdle, time.Now()
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	if code := e.run("move", handle, "other"); code != 0 {
		t.Fatalf("clalt move exit %d: %s", code, e.errOut.String())
	}

	// the fake writes the arguments first and the environment last
	r.waitUntil("claude to be started again with --resume", func() bool {
		raw, _ := os.ReadFile(r.file("session.env"))
		return strings.Contains(r.argv(), "--resume") && strings.Contains(string(raw), "CLAUDE_CONFIG_DIR="+e.folder("other")+"\n")
	})
	argv := strings.Split(strings.TrimSpace(r.argv()), "\n")
	if !contains(argv, "--resume") || !contains(argv, "claude-sess-1") || !contains(argv, "--settings") {
		t.Errorf("claude was started with %q, want --resume claude-sess-1 and the feed --settings", argv)
	}
	_, env, _ := e.claude.session()
	if env["CLAUDE_CONFIG_DIR"] != e.folder("other") {
		t.Errorf("claude ran with %q, want the folder of other, %q", env["CLAUDE_CONFIG_DIR"], e.folder("other"))
	}
	if env["CLALT_SESSION"] != handle {
		t.Errorf("CLALT_SESSION = %q, want the same handle %q", env["CLALT_SESSION"], handle)
	}
	rec, ok, _ := reg.Get(handle)
	if !ok || rec.Account != other.Name || rec.Folder != e.folder("other") {
		t.Errorf("record = %+v ok=%v, want it on %s at %s", rec, ok, other.Name, e.folder("other"))
	}
	if !strings.Contains(r.said(), "moved session") {
		t.Errorf("clalt did not say it moved the session: %s", r.said())
	}

	r.release()
	if code, ended := r.ended(10 * time.Second); !ended || code != 0 {
		t.Errorf("after claude ended: ended %v, exit %d: %s", ended, code, r.said())
	}
	if left := sessionRecordFiles(e); len(left) != 0 {
		t.Errorf("the session record was left behind: %v", left)
	}
}

func contains(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}
