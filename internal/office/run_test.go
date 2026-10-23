package office

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuf is a writer the server goroutine and the test can both touch.
type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// opens records what the opener was asked to open.
type opens struct {
	mu   sync.Mutex
	urls []string
}

func (o *opens) open(u string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.urls = append(o.urls, u)
	return nil
}

func (o *opens) list() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.urls...)
}

var errTest = errors.New("boom")

func testOptions(open func(string) error) Options {
	return Options{
		NoOpen: false, Open: open, Out: &syncBuf{},
		Snapshot: func() (Snapshot, error) {
			return Snapshot{V: 1, Accounts: []AccountOut{}, Sessions: []SessionOut{}}, nil
		},
	}
}

func TestListensOnLoopbackOnlyOnARandomPort(t *testing.T) {
	inst, err := Start(testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	tcp, ok := inst.Addr.(*net.TCPAddr)
	if !ok || !tcp.IP.Equal(net.IPv4(127, 0, 0, 1)) || tcp.Port == 0 {
		t.Fatalf("addr = %v, want 127.0.0.1 and a port", inst.Addr)
	}
}

func TestTheURLCarriesA128BitHexTokenThatDiffersPerRun(t *testing.T) {
	a, err := Start(testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Start(testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ta := a.URL[strings.Index(a.URL, "?t=")+3:]
	tb := b.URL[strings.Index(b.URL, "?t=")+3:]
	if len(ta) != 32 || ta == tb {
		t.Errorf("tokens %q %q", ta, tb)
	}
	if !strings.HasPrefix(a.URL, "http://127.0.0.1:") {
		t.Errorf("url = %s", a.URL)
	}
}

func TestTheServerAnswersOnItsOwnAddressAndRefusesWithoutTheToken(t *testing.T) {
	inst, err := Start(testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	base := "http://" + inst.Addr.String()
	resp, err := http.Get(base + "/api/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("no token = %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", base+"/api/snapshot", nil)
	req.Header.Set("X-Clalt-Token", inst.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"v":1`) {
		t.Errorf("with token = %d %s", resp.StatusCode, body)
	}
}

func TestAPortTakenIsAnError(t *testing.T) {
	inst, err := Start(testOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	opt := testOptions(nil)
	opt.Port = inst.Addr.(*net.TCPAddr).Port
	if _, err := Start(opt); err == nil {
		t.Error("starting on a taken port must fail")
	}
}

func TestRunOpensTheBrowserWithTheTokenURLUnlessNoOpen(t *testing.T) {
	rec := &opens{}
	opt := testOptions(rec.open)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opt) }()
	waitFor(t, func() bool { return len(rec.list()) == 1 })
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("clean stop returned %v", err)
	}
	opened := rec.list()
	if !strings.Contains(opened[0], "?t=") {
		t.Errorf("opened %q", opened[0])
	}
	if !strings.Contains(opt.Out.(*syncBuf).String(), opened[0]) {
		t.Errorf("the URL was not printed")
	}
}

func TestNoOpenDoesNotRunABrowser(t *testing.T) {
	rec := &opens{}
	opt := testOptions(rec.open)
	opt.NoOpen = true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opt) }()
	waitFor(t, func() bool { return strings.Contains(opt.Out.(*syncBuf).String(), "?t=") })
	cancel()
	<-done
	if len(rec.list()) != 0 {
		t.Error("--no-open ran the browser opener")
	}
}

func TestAFailedOpenOnlyWarnsTheURLStaysPrinted(t *testing.T) {
	opt := testOptions(func(string) error { return errTest })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opt) }()
	waitFor(t, func() bool { return strings.Contains(opt.Out.(*syncBuf).String(), "?t=") })
	cancel()
	if err := <-done; err != nil {
		t.Errorf("a failed browser open must not fail the command: %v", err)
	}
}

func TestOpenerCommandPerOS(t *testing.T) {
	cases := map[string][]string{
		"darwin":  {"open", "U"},
		"linux":   {"xdg-open", "U"},
		"windows": {"cmd", "/c", "start", "", "U"},
	}
	for goos, want := range cases {
		got, err := openerCommand(goos, "U")
		if err != nil || strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s = %v %v, want %v", goos, got, err, want)
		}
	}
	if _, err := openerCommand("plan9", "U"); err == nil {
		t.Error("an unknown OS must say so")
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out")
}
