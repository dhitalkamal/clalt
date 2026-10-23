package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// lockedOut is a writer the office goroutine and the test can both touch.
type lockedOut struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedOut) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedOut) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// startOffice runs `clalt office args` in the background with a context the
// test ends. open is the browser opener, a test never opens a real one.
func startOffice(t *testing.T, e *testEnv, open func(string) error, args ...string) (url string, stop func() int) {
	t.Helper()
	out := &lockedOut{}
	e.app.Out = out
	ctx, cancel := context.WithCancel(context.Background())
	e.app.officeCtx = ctx
	e.app.officeOpen = open
	done := make(chan int, 1)
	go func() { done <- e.run(append([]string{"office"}, args...)...) }()
	re := regexp.MustCompile(`http://127\.0\.0\.1:\d+/\?t=[0-9a-f]{32}`)
	for i := 0; i < 300 && url == ""; i++ {
		url = re.FindString(out.String())
		time.Sleep(10 * time.Millisecond)
	}
	if url == "" {
		cancel()
		t.Fatalf("no url printed: %q", out.String())
	}
	return url, func() int { cancel(); return <-done }
}

func fetch(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Clalt-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestOfficeIsInTheCommandTableAndTheHelp(t *testing.T) {
	e := newEnv(t)
	e.run("help")
	if !strings.Contains(e.out.String(), "clalt office") {
		t.Errorf("the help is missing clalt office:\n%s", e.out.String())
	}
	e.reset()
	e.run("office", "--bogus")
	if strings.Contains(e.errOut.String(), "unknown command") {
		t.Error("office is not in the command table")
	}
}

func TestOfficeRefusesBadFlags(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"--port"}, {"--port", "abc"}, {"--port", "70000"}, {"--port", "-1"}, {"extra"}} {
		e := newEnv(t)
		if code := e.run(append([]string{"office"}, args...)...); code != 2 || !strings.Contains(e.errOut.String(), "clalt office") {
			t.Errorf("%v: exit %d, err %q", args, code, e.errOut.String())
		}
	}
}

func TestOfficeServesTheSnapshotOfThisMachinesAccountsAndSessions(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	id := e.account("work").Name
	handle := "0123456789abcdef"
	reg := registry.Registry{Dir: config.SessionsDir()}
	if err := reg.Create(registry.Record{Handle: handle, Account: id, Cwd: "/home/me/Projects/clalt", WrapperPID: os.Getpid(), PID: os.Getpid(), StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	fd := feed.Store{Dir: config.FeedDir()}
	err := fd.UpdateSession(handle, id, time.Now(), func(s *feed.SessionState) bool { s.State = feed.StateBusy; s.Model = "claude-opus-5-5"; return true })
	if err != nil {
		t.Fatal(err)
	}
	opened := 0
	url, stop := startOffice(t, e, func(string) error { opened++; return nil }, "--no-open")
	base, token, _ := strings.Cut(url, "/?t=")

	if code, _ := fetch(t, base+"/api/snapshot", "wrong"); code != 401 {
		t.Errorf("wrong token = %d", code)
	}
	code, body := fetch(t, base+"/api/snapshot", token)
	if code != 200 {
		t.Fatalf("snapshot = %d %s", code, body)
	}
	var snap struct {
		V        int
		Accounts []struct{ ID, Label string }
		Sessions []struct{ Handle, Project, State, Model string }
	}
	if err := json.Unmarshal([]byte(body), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.V != 1 || len(snap.Accounts) != 1 || snap.Accounts[0].Label != "work" ||
		len(snap.Sessions) != 1 || snap.Sessions[0].Project != "clalt" || snap.Sessions[0].State != "busy" {
		t.Errorf("snapshot = %s", body)
	}
	if strings.Contains(body, "me@example.com") || strings.Contains(body, "/home/me") {
		t.Errorf("the snapshot leaks an email or a path: %s", body)
	}
	if code, page := fetch(t, base+"/", token); code != 200 || !strings.Contains(page, "<canvas") {
		t.Errorf("page = %d", code)
	}
	if code := stop(); code != 0 {
		t.Errorf("exit %d: %s", code, e.errOut.String())
	}
	if opened != 0 {
		t.Error("--no-open opened a browser")
	}
}

func TestOfficeOpensTheBrowserWithTheURLUnlessNoOpen(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	var got []string
	url, stop := startOffice(t, e, func(u string) error { mu.Lock(); got = append(got, u); mu.Unlock(); return nil })
	stop()
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != url {
		t.Errorf("opened %v, want [%s]", got, url)
	}
}

func TestOfficeLogsToTheLogFolder(t *testing.T) {
	if want := filepath.Join(config.LogsDir(), "office.log"); officeLogPath() != want {
		t.Errorf("log path %s, want %s", officeLogPath(), want)
	}
}

func TestOfficeShowsTheSubagentsTheFeedFileHolds(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "me@example.com")
	id := e.account("work").Name
	handle := "0123456789abcdef"
	reg := registry.Registry{Dir: config.SessionsDir()}
	if err := reg.Create(registry.Record{Handle: handle, Account: id, Cwd: "/p/app", WrapperPID: os.Getpid(), PID: os.Getpid(), StartedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	doc := `{"handle":"` + handle + `","account":"` + id + `","state":"busy","since":"2026-10-24T11:00:00Z","updatedAt":"2026-10-24T11:00:00Z",
		"subagents":[{"id":"a1","type":"Explore","label":"map it","state":"running","startedAt":"2026-10-24T11:00:00Z"}]}`
	p := filepath.Join(config.FeedDir(), "sessions", handle+".json")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	url, stop := startOffice(t, e, nil, "--no-open")
	base, token, _ := strings.Cut(url, "/?t=")
	_, body := fetch(t, base+"/api/snapshot", token)
	stop()
	if !strings.Contains(body, `"subagents":[{"id":"a1","type":"Explore","label":"map it","state":"running"`) {
		t.Errorf("snapshot has no subagents: %s", body)
	}
}
