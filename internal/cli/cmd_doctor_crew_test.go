package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/crew"
)

func crewChecksOf(t *testing.T, e *testEnv) []checkResult {
	t.Helper()
	e.reset()
	e.run("doctor", "--json")
	var rep doctorReport
	if err := json.Unmarshal(e.out.Bytes(), &rep); err != nil {
		t.Fatalf("%v: %s", err, e.out.String())
	}
	var out []checkResult
	for _, c := range rep.Checks {
		if strings.HasPrefix(c.Check, "crew") || c.Scope == "crew" {
			out = append(out, c)
		}
	}
	return out
}

func TestDoctorSaysNothingAboutCrewWhileThereIsNoGateway(t *testing.T) {
	e, k := crewEnv(t)
	if got := crewChecksOf(t, e); len(got) != 0 {
		t.Errorf("crew lines without a gateway: %+v", got)
	}
	if len(k.calls()) > 0 && k.calls()[0] != "" {
		t.Errorf("kirocrew was run: %v", k.calls())
	}
}

func TestDoctorReportsARunningGatewayAndTheKirocrewVersion(t *testing.T) {
	e, _ := crewEnv(t)
	e.run("crew", "start", "work")
	got := crewChecksOf(t, e)
	if len(got) != 2 {
		t.Fatalf("checks = %+v", got)
	}
	for _, c := range got {
		if c.Status != statusOK {
			t.Errorf("%+v", c)
		}
	}
}

func TestDoctorWarnsAboutAStaleRecordAndAnUnsupportedVersion(t *testing.T) {
	e, k := crewEnv(t)
	e.run("crew", "start", "work")
	rec, _, _ := crew.ReadRecord(e.account("work").Name)
	stopFake(t, k)
	waitGone(t, rec.PID)
	t.Setenv("FAKE_KIROCREW_VERSION", "0.10.0")
	var stale, version bool
	for _, c := range crewChecksOf(t, e) {
		if c.Status != statusWarn {
			t.Errorf("not a warning: %+v", c)
		}
		stale = stale || strings.Contains(c.Detail, "not running") && c.Scope == "work"
		version = version || strings.Contains(c.Detail, "0.10.0")
	}
	if !stale || !version {
		t.Errorf("stale=%v version=%v", stale, version)
	}
	if e.run("doctor") != 0 {
		t.Error("a stale gateway or a version out of range made doctor fail")
	}
}
