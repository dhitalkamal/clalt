package office

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

const sessionUUID = "11111111-2222-3333-4444-555555555555"

// realFixture is a Real over temp dirs with one transcript in a project folder.
func realFixture(t *testing.T, cwd string) (Real, string) {
	t.Helper()
	f := newFixture(t)
	projects := filepath.Join(t.TempDir(), "projects")
	dir := filepath.Join(projects, encodeCwd(cwd))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, sessionUUID+".jsonl")
	if err := os.WriteFile(main, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return Real{Registry: f.reg, Feed: f.feed, Projects: []string{projects}}, main
}

func addRun(t *testing.T, r Real, handle, claudeID, cwd string) {
	t.Helper()
	err := r.Registry.Create(registry.Record{Handle: handle, Account: "work", Cwd: cwd, ClaudeSessionID: claudeID})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTranscriptOfARunSessionByCwdSlug(t *testing.T) {
	r, main := realFixture(t, "/Users/x/Projects/my.app")
	addRun(t, r, runHandle, sessionUUID, "/Users/x/Projects/my.app")
	src, err := r.Transcript(runHandle, "")
	if err != nil || src.Path != main || src.Agent != "" {
		t.Errorf("src %+v err %v, want %s", src, err, main)
	}
}

func TestTranscriptFallsBackToAScanWhenTheSlugDoesNotMatch(t *testing.T) {
	r, main := realFixture(t, "/Users/x/Projects/a")
	addRun(t, r, runHandle, sessionUUID, "/somewhere/else/entirely")
	if src, err := r.Transcript(runHandle, ""); err != nil || src.Path != main {
		t.Errorf("src %+v err %v", src, err)
	}
}

func TestTranscriptOfAPlainClaudeSessionUsesTheFeedCwd(t *testing.T) {
	r, main := realFixture(t, "/Users/x/p")
	handle := "ext-" + sessionUUID
	err := r.Feed.UpdateSession(handle, "work", t0, func(s *feed.SessionState) bool { s.Cwd = "/Users/x/p"; return true })
	if err != nil {
		t.Fatal(err)
	}
	if src, err := r.Transcript(handle, ""); err != nil || src.Path != main {
		t.Errorf("src %+v err %v", src, err)
	}
}

func TestTranscriptSubagentFolderThenLegacyFilter(t *testing.T) {
	r, main := realFixture(t, "/Users/x/p")
	addRun(t, r, runHandle, sessionUUID, "/Users/x/p")
	// no per-session file: the sidechain lines of the main file
	src, err := r.Transcript(runHandle, "a111")
	if err != nil || src.Path != main || !src.Legacy || src.Agent != "a111" {
		t.Errorf("legacy src %+v err %v", src, err)
	}
	sub := filepath.Join(filepath.Dir(main), sessionUUID, "subagents")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "agent-a111.jsonl")
	if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err = r.Transcript(runHandle, "a111")
	if err != nil || src.Path != file || src.Legacy || src.Agent != "a111" {
		t.Errorf("folder src %+v err %v", src, err)
	}
}

func TestTranscriptNotFoundCases(t *testing.T) {
	r, _ := realFixture(t, "/Users/x/p")
	addRun(t, r, runHandle, "", "/Users/x/p")                                    // no claude id yet
	addRun(t, r, deadHand, "99999999-2222-3333-4444-555555555555", "/Users/x/p") // no file
	addRun(t, r, "aaaaaaaaaaaaaaaa", "../../etc/passwd", "/Users/x/p")           // unsafe id
	for _, h := range []string{runHandle, deadHand, "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb", "ext-99999999-2222-3333-4444-555555555555"} {
		_, err := r.Transcript(h, "")
		var nf *NotFoundError
		if !errors.As(err, &nf) || nf.Reason == "" {
			t.Errorf("%s: err = %v, want a NotFoundError with a reason", h, err)
		}
	}
}
