package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// the office answers /api/summary and /api/history from the transcripts and
// the session ledger of this machine, by account.
func TestOfficeServesTheSummaryAndHistoryByAccount(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	id := e.account("work").Name
	now := time.Now().Add(2 * time.Second) // after the sign-in, whole seconds in the line
	dir := filepath.Join(config.ClaudeProjectsDir(), "-home-me-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"type":"assistant","timestamp":%q,"sessionId":"sess-1","cwd":"/home/me/demo","requestId":"r1","message":{"id":"m1","model":"claude-sonnet-4-5","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/home/me/demo/a.txt"}}],"usage":{"input_tokens":40,"output_tokens":2}}}`+"\n", now.UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(dir, "sess-1.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	err := usage.AppendSessionAccount(config.SessionAccountsPath(), usage.SessionAccount{Ts: now.Add(-time.Hour), SessionID: "sess-1", Account: id})
	if err != nil {
		t.Fatal(err)
	}
	url, stop := startOffice(t, e, func(string) error { return nil }, "--no-open")
	base, token, _ := strings.Cut(url, "/?t=")

	code, body := fetch(t, base+"/api/summary", token)
	if code != 200 {
		t.Fatalf("summary = %d %s", code, body)
	}
	var sum struct {
		Totals   struct{ Tokens int64 }
		Accounts []struct {
			Label    string
			Sessions int
			Projects []struct {
				Name  string
				Files []struct{ Name string }
			}
		}
	}
	if err := json.Unmarshal([]byte(body), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Totals.Tokens != 42 || len(sum.Accounts) != 1 || sum.Accounts[0].Label != "work" || sum.Accounts[0].Sessions != 1 ||
		sum.Accounts[0].Projects[0].Name != "demo" || sum.Accounts[0].Projects[0].Files[0].Name != "a.txt" {
		t.Errorf("summary = %s", body)
	}
	if strings.Contains(body, "me@example.com") || strings.Contains(body, "/home/me") {
		t.Errorf("the summary leaks an email or a path: %s", body)
	}
	code, body = fetch(t, base+"/api/history?days=3", token)
	if code != 200 || !strings.Contains(body, `"tokens":42`) {
		t.Errorf("history = %d %s", code, body)
	}
	if code := stop(); code != 0 {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}

// the readings log of the feed fills the percents of /api/history and the
// peak of /api/summary
func TestOfficeServesPercentHistoryFromTheReadingsLog(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	id := e.account("work").Name
	now := time.Now()
	st := feed.Store{Dir: config.FeedDir()}
	// an old reading, as a day ago, written straight to the log
	old := feed.Reading{T: now.Add(-26 * time.Hour)}
	v, r := 73.0, now.Add(time.Hour)
	old.FiveHour, old.FiveHourResetsAt = &v, &r
	raw, _ := json.Marshal(old)
	p := filepath.Join(config.FeedDir(), "readings", id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.MergeLimits(id, &feed.Window{UsedPct: 20, ResetsAt: now.Add(time.Hour)}, nil, now); err != nil {
		t.Fatal(err)
	}
	url, stop := startOffice(t, e, func(string) error { return nil }, "--no-open")
	base, token, _ := strings.Cut(url, "/?t=")
	code, body := fetch(t, base+"/api/history?days=3", token)
	if code != 200 || !strings.Contains(body, `"peak5hPct":73`) || !strings.Contains(body, `"peak5hPct":20`) {
		t.Errorf("history = %d %s", code, body)
	}
	if code := stop(); code != 0 {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
}
