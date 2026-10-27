package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
)

// clalt mcp answers on stdout with protocol lines only, from the accounts and
// the feed of this machine.
func TestMcpServesTheLimitsOfTheMachine(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	e.at(theClock)
	id := e.account("work").Name
	fd := feed.Store{Dir: config.FeedDir()}
	if err := fd.MergeLimits(id, &feed.Window{UsedPct: 42, ResetsAt: theClock.Add(time.Hour)}, nil, theClock); err != nil {
		t.Fatal(err)
	}
	e.app.In = strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n" +
			`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"limits","arguments":{}}}` + "\n")
	if code := e.run("mcp"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut)
	}
	lines := strings.Split(strings.TrimSpace(e.out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want two replies", e.out.String())
	}
	for _, l := range lines {
		if !json.Valid([]byte(l)) {
			t.Errorf("stdout line is not json: %q", l)
		}
	}
	if !strings.Contains(lines[1], `work 5h 42%`) || strings.Contains(e.out.String(), "me@example.com") {
		t.Errorf("limits reply = %s", lines[1])
	}
}

func TestMcpTakesNoArguments(t *testing.T) {
	e := newEnv(t)
	if code := e.run("mcp", "--port", "1"); code != 2 {
		t.Errorf("exit = %d, want 2: %s", code, e.errOut)
	}
	if !strings.Contains(e.errOut.String(), "clalt mcp") {
		t.Errorf("stderr = %q", e.errOut)
	}
}

// --json is the global flag and must not change what mcp prints
func TestMcpSessionsAndTodayAreReachable(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	e.app.In = strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sessions","arguments":{}}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"today","arguments":{}}}` + "\n")
	if code := e.run("mcp"); code != 0 {
		t.Fatalf("exit %d: %s", code, e.errOut)
	}
	if strings.Contains(e.out.String(), `"isError":true`) || strings.Count(e.out.String(), "\n") != 2 {
		t.Errorf("stdout = %s", e.out)
	}
}
