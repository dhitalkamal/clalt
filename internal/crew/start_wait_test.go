package crew

import (
	"context"
	"errors"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestStartWaitsUntilTheGatewayAnswers(t *testing.T) {
	w := newWorld(t)
	var asked int
	w.ready = func(int) bool { asked++; return asked >= 4 }
	res, err := start(t, w, "work")
	if err != nil || res.Warning != "" {
		t.Fatalf("err = %v, warning = %q", err, res.Warning)
	}
	if asked < 4 {
		t.Errorf("the port was asked about %d times", asked)
	}
}

func TestAGatewayThatIsAliveButSilentIsAWarningAndNotAFailure(t *testing.T) {
	w := newWorld(t)
	w.ready = func(int) bool { return false }
	res, err := start(t, w, "work")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.Warning, "5500") || !strings.Contains(res.Warning, "logs") {
		t.Errorf("Warning = %q", res.Warning)
	}
	if _, found, _ := ReadRecord("work"); !found {
		t.Error("the gateway that is running has no record")
	}
}

func TestAGatewayThatDiesAtOnceIsAnErrorWithTheEndOfItsOutput(t *testing.T) {
	w := newWorld(t)
	w.diesAtOnce = true
	w.logText = "starting\naddress already in use\n"
	_, err := start(t, w, "work")
	var fail *StartFailure
	if !errors.As(err, &fail) {
		t.Fatalf("err = %v, want a *StartFailure", err)
	}
	if !strings.Contains(fail.Reason, "exited") || !slices.Equal(fail.Output, []string{"starting", "address already in use"}) {
		t.Errorf("failure = %+v", fail)
	}
	if _, found, _ := ReadRecord("work"); found {
		t.Error("a gateway that never ran has a record")
	}
}

func TestAGatewayThatDiesWhileStartingIsAnErrorAndLeavesNoRecord(t *testing.T) {
	w := newWorld(t)
	w.logText = "loading\ncrashed\n"
	w.ready = func(int) bool { return false }
	w.onPS = func(pid, nth int) {
		if nth == 3 {
			w.kill(pid)
		}
	}
	_, err := start(t, w, "work")
	var fail *StartFailure
	if !errors.As(err, &fail) || !strings.Contains(fail.Reason, "exited") || len(fail.Output) != 2 {
		t.Fatalf("err = %v", err)
	}
	if _, found, _ := ReadRecord("work"); found {
		t.Error("the record of a dead gateway was left")
	}
}

func TestTheOutputOfAFailedStartNeverShowsATokenValue(t *testing.T) {
	w := newWorld(t)
	w.diesAtOnce = true
	w.logText = "open http://localhost:5500/?token=abc123secret to continue\n"
	_, err := start(t, w, "work")
	var fail *StartFailure
	if !errors.As(err, &fail) {
		t.Fatalf("err = %v", err)
	}
	if got := strings.Join(fail.Output, "\n"); strings.Contains(got, "abc123secret") || !strings.Contains(got, "token=") {
		t.Errorf("output = %q", got)
	}
}

func TestAGatewayThatCannotBeLaunchedIsAnErrorWithNoRecord(t *testing.T) {
	w := newWorld(t)
	w.startErr = errors.New("fork failed")
	if _, err := start(t, w, "work"); err == nil || !strings.Contains(err.Error(), "fork failed") {
		t.Errorf("err = %v", err)
	}
	if _, found, _ := ReadRecord("work"); found {
		t.Error("a record was written")
	}
}

func TestWhenPsCannotBeReadTheGatewayJustLaunchedIsStoppedAgainAndNotLeftUnrecorded(t *testing.T) {
	w := newWorld(t)
	w.psBroken = true
	_, err := start(t, w, "work")
	if err == nil || !strings.Contains(err.Error(), "ps") {
		t.Fatalf("err = %v", err)
	}
	if !slices.Equal(w.signals, []int{4242}) {
		t.Errorf("signals = %v, want the pid that was just launched, and only that", w.signals)
	}
	if _, found, _ := ReadRecord("work"); found {
		t.Error("a record was written for a gateway that could not be identified")
	}
}

func TestStartOfARunningGatewayChangesNothing(t *testing.T) {
	w := newWorld(t)
	first, err := start(t, w, "work")
	if err != nil {
		t.Fatal(err)
	}
	calls := len(w.calls)
	again, err := start(t, w, "work")
	if err != nil || !again.Already || again.Record != first.Record {
		t.Fatalf("again = %+v, %v", again, err)
	}
	if len(w.started) != 1 || len(w.calls) != calls {
		t.Errorf("started %d gateways and ran %d more commands", len(w.started), len(w.calls)-calls)
	}
}

func TestStartReplacesARecordWhoseGatewayIsGone(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	w.kill(4242)
	res, err := start(t, w, "work")
	if err != nil || res.Already || res.Record.PID != 4243 {
		t.Fatalf("res = %+v, %v", res, err)
	}
	if got, _, _ := ReadRecord("work"); got.PID != 4243 {
		t.Errorf("the record still names pid %d", got.PID)
	}
}

func TestStartReplacesARecordWhosePidIsAnotherProcessNow(t *testing.T) {
	w := newWorld(t)
	if err := WriteRecord("work", Record{PID: 700, Port: 5500, ProcStart: psStart}); err != nil {
		t.Fatal(err)
	}
	w.plant(700, "Mon Oct 26 09:00:00 2026", "/usr/bin/vim notes.txt")
	res, err := start(t, w, "work")
	if err != nil || res.Record.PID == 700 {
		t.Fatalf("res = %+v, %v", res, err)
	}
	if len(w.signals) != 0 {
		t.Errorf("signals were sent: %v", w.signals)
	}
}

func TestStartRefusesWhenTheRecordedProcessCannotBeConfirmed(t *testing.T) {
	w := newWorld(t)
	if err := WriteRecord("work", Record{PID: 700, Port: 5500, ProcStart: psStart}); err != nil {
		t.Fatal(err)
	}
	w.plant(700, psStart, "/usr/bin/vim notes.txt")
	_, err := start(t, w, "work")
	if err == nil || !strings.Contains(err.Error(), "700") {
		t.Fatalf("err = %v", err)
	}
	if len(w.started) != 0 {
		t.Error("a second gateway was started over an unclear one")
	}
	if _, found, _ := ReadRecord("work"); !found {
		t.Error("the record was removed")
	}
}

func TestStartRefusesARecordItCannotRead(t *testing.T) {
	w := newWorld(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Record, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := start(t, w, "work"); err == nil || !strings.Contains(err.Error(), p.Record) {
		t.Errorf("err = %v", err)
	}
	if len(w.started) != 0 {
		t.Error("a gateway was started with a record nobody can read")
	}
}

func TestAFolderThatIsNotAbsoluteIsRefused(t *testing.T) {
	w := newWorld(t)
	for _, folder := range []string{"", "accounts/work"} {
		if _, err := w.manager().Start(context.Background(), StartRequest{Account: "work", Folder: folder}); err == nil {
			t.Errorf("folder %q was accepted", folder)
		}
	}
	if len(w.started) != 0 {
		t.Error("a gateway was started")
	}
}

func TestStartWithNoHomeIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows falls back to USERPROFILE, so there is always a home")
	}
	w := newWorld(t)
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if _, err := start(t, w, "work"); err == nil {
		t.Error("no error")
	}
	if len(w.started) != 0 {
		t.Error("a gateway was started")
	}
}

func TestStartsAtTheSameTimeNeverGetTheSamePort(t *testing.T) {
	w := newWorld(t)
	ids := []string{"a", "b", "c", "d"}
	var wg sync.WaitGroup
	errs := make([]error, len(ids))
	for i, id := range ids {
		folder := w.folder(id)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = w.manager().Start(context.Background(), StartRequest{Account: id, Folder: folder})
		}()
	}
	wg.Wait()
	ports := map[int]string{}
	for i, id := range ids {
		if errs[i] != nil {
			t.Fatalf("%s: %v", id, errs[i])
		}
		port := recordedPortOf(t, id)
		if other, dup := ports[port]; dup {
			t.Errorf("%s and %s both got port %d", id, other, port)
		}
		ports[port] = id
	}
}
