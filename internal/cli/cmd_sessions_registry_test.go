package cli

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/registry"
)

func TestSessionsJSONGrowsTheRegistryFieldsAndKeepsTheOldOnes(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	h := registry.NewHandle()
	err := registry.Registry{Dir: config.SessionsDir()}.Create(registry.Record{
		Handle: h, Account: "work", Folder: "/f/work", WrapperPID: os.Getpid(), PID: os.Getpid(),
		ClaudeSessionID: "sess-1", Cwd: "/code", StartedAt: time.Now(),
		Terminal: registry.Terminal{Kind: "kitty", KittyWindowID: "4"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if code := e.run("sessions", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("%v\n%s", err, e.out.String())
	}
	r := rows[0]
	for k, want := range map[string]any{"account": "work", "label": "work", "cwd": "/code", "handle": h,
		"folder": "/f/work", "claudeSessionId": "sess-1", "wrapperPid": float64(os.Getpid())} {
		if r[k] != want {
			t.Errorf("%s = %v, want %v", k, r[k], want)
		}
	}
	term, _ := r["terminal"].(map[string]any)
	if term["kind"] != "kitty" || term["kittyWindowId"] != "4" {
		t.Errorf("terminal = %v", r["terminal"])
	}
}
