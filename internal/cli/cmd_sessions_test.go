package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/profile"
)

// liveSession records a session of the account with this id, owned by the test
// process so that it counts as alive.
func liveSession(t *testing.T, id, cwd string) {
	t.Helper()
	err := profile.WriteSession(config.SessionsDir(), profile.Session{
		Account: id, PID: os.Getpid(), Cwd: cwd, StartedAt: time.Now().Add(-90 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSessionsShowTheLabelNotTheId(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if err := e.app.Store.Relabel("work", "office"); err != nil {
		t.Fatal(err)
	}
	liveSession(t, "work", "/code/clalt")

	code := e.run("sessions")

	out := e.out.String()
	if code != 0 || !strings.Contains(out, "office") || !strings.Contains(out, "/code/clalt") || strings.Contains(out, "work ") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

func TestSessionsJSONCarriesBothTheIdAndTheLabel(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	if err := e.app.Store.Relabel("work", "office"); err != nil {
		t.Fatal(err)
	}
	liveSession(t, "work", "/code/clalt")

	if code := e.run("sessions", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("%v\n%s", err, e.out.String())
	}
	if rows[0]["account"] != "work" || rows[0]["label"] != "office" || rows[0]["cwd"] != "/code/clalt" || rows[0]["pid"] == nil {
		t.Errorf("row = %v", rows[0])
	}
}

func TestSessionsOfAnAccountClaltDoesNotKnowKeepTheirName(t *testing.T) {
	e := newEnv(t)
	liveSession(t, "stray", "/code")

	e.run("sessions")

	if !strings.Contains(e.out.String(), "stray") {
		t.Errorf("output:\n%s", e.out.String())
	}
}

func TestSessionsWithNoneRunningSaysSo(t *testing.T) {
	e := newEnv(t)

	if code := e.run("sessions"); code != 0 || !strings.Contains(e.out.String(), "no running sessions") {
		t.Errorf("exit %d:\n%s", code, e.out.String())
	}
	e.reset()
	e.run("sessions", "--json")
	if strings.TrimSpace(e.out.String()) != "[]" {
		t.Errorf("json = %q, want an empty list", e.out.String())
	}
}

func TestSessionsTakeNoArguments(t *testing.T) {
	e := newEnv(t)
	if code := e.run("sessions", "work"); code != 2 || !strings.Contains(e.errOut.String(), "usage: clalt sessions") {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}
