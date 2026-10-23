package crew

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStatusOfARunningGatewayHasItsRecordAndUptime(t *testing.T) {
	w := newWorld(t)
	started, err := start(t, w, "work")
	if err != nil {
		t.Fatal(err)
	}
	m := w.manager()
	m.o.Now = func() time.Time { return started.Record.StartedAt.Add(2*time.Hour + 5*time.Minute) }
	st, err := m.Status(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateRunning || st.Record != started.Record || st.Uptime != 2*time.Hour+5*time.Minute || st.Why != "" {
		t.Errorf("Status = %+v", st)
	}
	p, _ := PathsFor("work")
	if st.Paths != p {
		t.Errorf("Paths = %+v", st.Paths)
	}
}

func TestStatusOfAnAccountWithNoRecordIsStopped(t *testing.T) {
	w := newWorld(t)
	st, err := w.manager().Status(context.Background(), "work")
	if err != nil || st.State != StateStopped || st.Account != "work" {
		t.Errorf("Status = %+v, %v", st, err)
	}
}

func TestStatusSaysWhyAGatewayIsNotRunning(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "gone"); err != nil {
		t.Fatal(err)
	}
	w.kill(4242)
	if err := WriteRecord("unclear", Record{PID: 700, Port: 5481, ProcStart: psStart}); err != nil {
		t.Fatal(err)
	}
	w.plant(700, psStart, "vim")
	for id, want := range map[string]State{"gone": StateStale, "unclear": StateUnverified} {
		st, err := w.manager().Status(context.Background(), id)
		if err != nil || st.State != want || st.Why == "" {
			t.Errorf("%s: Status = %+v, %v, want %s with a reason", id, st, err, want)
		}
	}
}

func TestStatusOfARecordThatCannotBeReadIsBroken(t *testing.T) {
	w := newWorld(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Record, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := w.manager().Status(context.Background(), "work")
	if err != nil || st.State != StateBroken || !strings.Contains(st.Why, p.Record) {
		t.Errorf("Status = %+v, %v", st, err)
	}
}

func TestStatusOnlyLooksAndNeverChangesAnything(t *testing.T) {
	w := newWorld(t)
	if _, err := start(t, w, "work"); err != nil {
		t.Fatal(err)
	}
	w.kill(4242)
	m := w.manager()
	if _, err := m.Status(context.Background(), "work"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.StatusAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := ReadRecord("work"); !found {
		t.Error("a stale record was removed by looking at it")
	}
	if len(w.signals) != 0 {
		t.Errorf("signals = %v", w.signals)
	}
}

func TestStatusAllListsEveryRecordSortedByAccount(t *testing.T) {
	w := newWorld(t)
	for _, id := range []string{"zed", "alpha", "mid"} {
		if _, err := start(t, w, id); err != nil {
			t.Fatal(err)
		}
	}
	w.kill(4244) // started third: mid
	all, err := w.manager().StatusAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, st := range all {
		got = append(got, st.Account+":"+string(st.State))
	}
	if want := "alpha:running,mid:stale,zed:running"; strings.Join(got, ",") != want {
		t.Errorf("StatusAll = %v, want %s", got, want)
	}
}

func TestStatusAllWithNoGatewaysIsEmpty(t *testing.T) {
	w := newWorld(t)
	if all, err := w.manager().StatusAll(context.Background()); err != nil || len(all) != 0 {
		t.Errorf("StatusAll = %v, %v", all, err)
	}
}

func TestStatusOfAnIdThatCannotBeAFolderIsAnError(t *testing.T) {
	w := newWorld(t)
	if _, err := w.manager().Status(context.Background(), "../x"); err == nil {
		t.Error("no error")
	}
}
