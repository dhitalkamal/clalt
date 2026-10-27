package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/registry"
)

// feedLines are the doctor lines about the feed, by check name.
func feedLines(t *testing.T, e *testEnv) map[string]checkResult {
	t.Helper()
	e.reset()
	e.run("doctor", "--json")
	var rep doctorReport
	if err := json.Unmarshal(e.out.Bytes(), &rep); err != nil {
		t.Fatalf("%v: %s", err, e.out.String())
	}
	out := map[string]checkResult{}
	for _, c := range rep.Checks {
		if c.Scope == "feed" {
			out[c.Check] = c
		}
	}
	return out
}

func TestDoctorFeedIsQuietWithNothingRunning(t *testing.T) {
	e := newEnv(t)

	got := feedLines(t, e)

	if got["sessions"].Status != statusOK || got["hookErrors"].Status != statusOK {
		t.Errorf("%+v", got)
	}
}

func TestDoctorFeedWarnsAboutRecordsAndFilesOfSessionsThatAreGoneAndChangesNothing(t *testing.T) {
	e := newEnv(t)
	dead := registry.NewHandle()
	reg := registry.Registry{Dir: config.SessionsDir()}
	_ = reg.Create(registry.Record{Handle: dead, Account: "work", WrapperPID: 2147483646, StartedAt: time.Now()})
	orphan := registry.NewHandle()
	_ = feedStore().UpdateSession(orphan, "work", time.Now().Add(-time.Hour), func(*feed.SessionState) bool { return true })

	got := feedLines(t, e)

	if got["sessions"].Status != statusWarn || !strings.Contains(got["sessions"].Detail, "gone") {
		t.Errorf("%+v", got["sessions"])
	}
	if _, ok, _ := reg.Get(dead); !ok {
		t.Error("doctor removed a record")
	}
	if _, ok := feedStore().Session(orphan); !ok {
		t.Error("doctor removed a feed file")
	}
}

func TestDoctorFeedWarnsWhenALiveSessionNeverReported(t *testing.T) {
	e := newEnv(t)
	e.signIn("work", "w@example.com")
	h := registry.NewHandle()
	_ = registry.Registry{Dir: config.SessionsDir()}.Create(registry.Record{
		Handle: h, Account: "work", WrapperPID: os.Getpid(), StartedAt: time.Now().Add(-10 * time.Minute),
	})
	_ = feedStore().UpdateSession(h, "work", time.Now().Add(-10*time.Minute), func(*feed.SessionState) bool { return true })

	got := feedLines(t, e)["sessions"]

	if got.Status != statusWarn || !strings.Contains(got.Detail, "has not reported") || !strings.Contains(got.Detail, "10m") {
		t.Errorf("%+v", got)
	}
}

func TestDoctorFeedIsOkForReportingSessions(t *testing.T) {
	e := newEnv(t)
	liveHandle(t)
	e.hook("SessionStart", `{"session_id":"s"}`)

	got := feedLines(t, e)["sessions"]

	if got.Status != statusOK || !strings.Contains(got.Detail, "1 running") {
		t.Errorf("%+v", got)
	}
}

func TestDoctorFeedCountsHookErrorsOfTheLastDayOnly(t *testing.T) {
	e := newEnv(t)
	now := time.Now().UTC()
	if err := os.MkdirAll(config.LogsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	log := strings.Join([]string{
		now.Add(-48*time.Hour).Format(time.RFC3339) + " Stop abc: an old one",
		now.Add(-2*time.Hour).Format(time.RFC3339) + " Stop abc: first",
		now.Add(-time.Hour).Format(time.RFC3339) + " SessionStart abc: update the feed: timed out",
		"garbage without a time",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(config.LogsDir(), "hook.log"), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}

	got := feedLines(t, e)["hookErrors"]

	if got.Status != statusWarn || !strings.Contains(got.Detail, "2 hook errors") || !strings.Contains(got.Detail, "timed out") {
		t.Errorf("%+v", got)
	}
}
