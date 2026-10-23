package crew

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func stop(t *testing.T, w *world, id string) (StopResult, error) {
	t.Helper()
	return w.manager().Stop(context.Background(), id)
}

func TestStopEndsTheGatewayThatWasStartedAndRemovesItsRecord(t *testing.T) {
	w := newWorld(t)
	started, err := start(t, w, "work")
	if err != nil {
		t.Fatal(err)
	}
	res, err := stop(t, w, "work")
	if err != nil || res.Outcome != Stopped || res.Record != started.Record {
		t.Fatalf("res = %+v, %v", res, err)
	}
	if !slices.Equal(w.signals, []int{4242}) {
		t.Errorf("signals = %v", w.signals)
	}
	if _, found, _ := ReadRecord("work"); found {
		t.Error("the record is still there")
	}
}

func TestStopOnlyEverSignalsThePidOfItsOwnAccount(t *testing.T) {
	w := newWorld(t)
	for _, id := range []string{"a", "b"} {
		if _, err := start(t, w, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stop(t, w, "b"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(w.signals, []int{4243}) {
		t.Errorf("signals = %v, want only the pid of b", w.signals)
	}
	if _, found, _ := ReadRecord("a"); !found {
		t.Error("the record of a went with it")
	}
}

func TestStopOfAnAccountThatNeverRanSignalsNothing(t *testing.T) {
	w := newWorld(t)
	res, err := stop(t, w, "work")
	if err != nil || res.Outcome != NotRunning {
		t.Errorf("res = %+v, %v", res, err)
	}
	if len(w.signals) != 0 {
		t.Errorf("signals = %v", w.signals)
	}
}

func TestStopClearsARecordWhoseGatewayIsGone(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	w.kill(4242)
	res, err := stop(t, w, "work")
	if err != nil || res.Outcome != Cleared || !strings.Contains(res.Why, "4242") {
		t.Fatalf("res = %+v, %v", res, err)
	}
	if len(w.signals) != 0 {
		t.Errorf("signals = %v", w.signals)
	}
	if _, found, _ := ReadRecord("work"); found {
		t.Error("the stale record is still there")
	}
}

func TestStopNeverSignalsAPidThatIsAnotherProcessNow(t *testing.T) {
	for _, pid := range []int{700, 1} {
		w := newWorld(t)
		if err := WriteRecord("work", Record{PID: pid, Port: 5480, ProcStart: psStart}); err != nil {
			t.Fatal(err)
		}
		w.plant(pid, "Mon Oct 26 09:00:00 2026", "/sbin/launchd")
		res, err := stop(t, w, "work")
		if err != nil || res.Outcome != Cleared || !strings.Contains(res.Why, "another process") {
			t.Fatalf("pid %d: res = %+v, %v", pid, res, err)
		}
		if len(w.signals) != 0 {
			t.Errorf("pid %d: signals = %v", pid, w.signals)
		}
	}
}

func TestStopRefusesAProcessThatStartedWithTheGatewayButIsNotOne(t *testing.T) {
	w := newWorld(t)
	if err := WriteRecord("work", Record{PID: 700, Port: 5480, ProcStart: psStart}); err != nil {
		t.Fatal(err)
	}
	w.plant(700, psStart, "/usr/bin/vim notes.txt")
	_, err := stop(t, w, "work")
	if err == nil || !strings.Contains(err.Error(), "700") || !strings.Contains(err.Error(), "will not signal") {
		t.Fatalf("err = %v", err)
	}
	if len(w.signals) != 0 {
		t.Errorf("signals = %v", w.signals)
	}
	if _, found, _ := ReadRecord("work"); !found {
		t.Error("the record was removed")
	}
}

func TestStopRefusesARecordThatKeptNoStartTime(t *testing.T) {
	w := newWorld(t)
	if err := WriteRecord("work", Record{PID: 700, Port: 5480}); err != nil {
		t.Fatal(err)
	}
	w.plant(700, psStart, "/App/python3.12 -m kiro_crew gateway --port 5480")
	if _, err := stop(t, w, "work"); err == nil {
		t.Error("a process that cannot be told from another was signalled")
	}
	if len(w.signals) != 0 {
		t.Errorf("signals = %v", w.signals)
	}
}

func TestWhenPsCannotBeReadStopSignalsNothing(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	w.psBroken = true
	if _, err := stop(t, w, "work"); err == nil {
		t.Error("no error")
	}
	if len(w.signals) != 0 {
		t.Errorf("signals = %v", w.signals)
	}
	if _, found, _ := ReadRecord("work"); !found {
		t.Error("the record was removed")
	}
}

func TestStopRefusesARecordItCannotRead(t *testing.T) {
	w := newWorld(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Record, []byte("[1,2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := stop(t, w, "work"); err == nil || !strings.Contains(err.Error(), p.Record) {
		t.Errorf("err = %v", err)
	}
	if len(w.signals) != 0 {
		t.Errorf("signals = %v", w.signals)
	}
}

func TestStopWaitsForTheGatewayToEnd(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	w.ignoresTerm = true
	asked := 0
	w.onPS = func(pid, nth int) {
		asked++
		if asked == 4 {
			w.kill(pid)
		}
	}
	res, err := stop(t, w, "work")
	if err != nil || res.Outcome != Stopped {
		t.Fatalf("res = %+v, %v", res, err)
	}
	if len(w.signals) != 1 {
		t.Errorf("the gateway was signalled %d times", len(w.signals))
	}
}

func TestAGatewayThatDoesNotEndIsReportedAndKeepsItsRecord(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	w.ignoresTerm = true
	_, err := stop(t, w, "work")
	if err == nil || !strings.Contains(err.Error(), "4242") || !strings.Contains(err.Error(), "still running") {
		t.Fatalf("err = %v", err)
	}
	if _, found, _ := ReadRecord("work"); !found {
		t.Error("the record of a gateway that is still running was removed")
	}
	if len(w.signals) != 1 {
		t.Errorf("the gateway was signalled %d times, and never killed", len(w.signals))
	}
}

func TestASignalThatFailsIsReportedAndKeepsTheRecord(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	m := w.manager()
	m.o.Signal = func(int) error { return errors.New("operation not permitted") }
	if _, err := m.Stop(context.Background(), "work"); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Errorf("err = %v", err)
	}
	if _, found, _ := ReadRecord("work"); !found {
		t.Error("the record was removed")
	}
}
