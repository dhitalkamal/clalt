package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const deadPID = 2147483646

func rec(handle string, wrapper int) Record {
	return Record{Handle: handle, Account: "work", Folder: "/f", WrapperPID: wrapper, Cwd: "/c",
		StartedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}
}

func newReg(t *testing.T) Registry { return Registry{Dir: filepath.Join(t.TempDir(), "sessions")} }

func TestCreateThenGetAndRemove(t *testing.T) {
	r := newReg(t)
	h := NewHandle()
	if err := r.Create(rec(h, os.Getpid())); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.Get(h)
	if err != nil || !ok || got.Account != "work" || got.WrapperPID != os.Getpid() {
		t.Fatalf("got %+v %v %v", got, ok, err)
	}
	if err := r.Remove(h); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.Get(h); ok {
		t.Error("still there after Remove")
	}
	if err := r.Remove(h); err != nil {
		t.Errorf("removing a missing record: %v", err)
	}
}

func TestGetOfABadHandleIsNotThereAndNeverTouchesOtherFiles(t *testing.T) {
	r := newReg(t)
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.Get("../x"); ok || err != nil {
		t.Errorf("ok=%v err=%v", ok, err)
	}
	if err := r.Create(rec("../x", 1)); err == nil {
		t.Error("created a record under a bad handle")
	}
}

func TestUpdateChangesTheRecordAndMissingIsReported(t *testing.T) {
	r := newReg(t)
	h := NewHandle()
	if err := r.Create(rec(h, os.Getpid())); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.Update(h, func(x *Record) { x.PID = 42; x.ClaudeSessionID = "abc" })
	if err != nil || !ok || got.PID != 42 {
		t.Fatalf("%+v %v %v", got, ok, err)
	}
	again, _, _ := r.Get(h)
	if again.PID != 42 || again.ClaudeSessionID != "abc" || again.Account != "work" {
		t.Errorf("stored %+v", again)
	}
	if _, ok, err := r.Update(NewHandle(), func(*Record) {}); ok || err != nil {
		t.Errorf("missing: ok=%v err=%v", ok, err)
	}
	// a record that was removed is not brought back by a late update
	_ = r.Remove(h)
	if _, ok, _ := r.Update(h, func(x *Record) { x.PID = 1 }); ok {
		t.Error("update recreated a removed record")
	}
	if _, ok, _ := r.Get(h); ok {
		t.Error("record is back")
	}
}

func TestListKeepsLiveRecordsOldestFirstAndPrunesDeadOnes(t *testing.T) {
	r := newReg(t)
	live, dead, old := NewHandle(), NewHandle(), NewHandle()
	a := rec(live, os.Getpid())
	b := rec(dead, deadPID)
	c := rec(old, os.Getpid())
	c.StartedAt = a.StartedAt.Add(-time.Hour)
	for _, x := range []Record{a, b, c} {
		if err := r.Create(x); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.List()
	if err != nil || len(got) != 2 || got[0].Handle != old || got[1].Handle != live {
		t.Fatalf("%+v %v", got, err)
	}
	if _, ok, _ := r.Get(dead); ok {
		t.Error("the dead record was not pruned")
	}
}

func TestScanReportsLivenessAndChangesNothing(t *testing.T) {
	r := newReg(t)
	dead := NewHandle()
	_ = r.Create(rec(dead, deadPID))
	_ = os.WriteFile(filepath.Join(r.Dir, "junk.json"), []byte("{not json"), 0o600)
	entries, bad, err := r.Scan()
	if err != nil || len(entries) != 1 || entries[0].Alive || bad != 1 {
		t.Fatalf("%+v bad=%d %v", entries, bad, err)
	}
	if _, ok, _ := r.Get(dead); !ok {
		t.Error("Scan removed a record")
	}
}

func TestAV02RecordWithoutAWrapperIsLiveByItsPID(t *testing.T) {
	r := newReg(t)
	raw := []byte(`{"account":"a","pid":` + itoa(os.Getpid()) + `,"cwd":"/x","startedAt":"2026-01-01T00:00:00Z"}`)
	_ = os.MkdirAll(r.Dir, 0o700)
	_ = os.WriteFile(filepath.Join(r.Dir, itoa(os.Getpid())+".json"), raw, 0o600)
	got, err := r.List()
	if err != nil || len(got) != 1 || got[0].Account != "a" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestRemovePIDRemovesTheRecordsOfThatProcess(t *testing.T) {
	r := newReg(t)
	a, b := NewHandle(), NewHandle()
	x := rec(a, os.Getpid())
	x.PID = 777
	_ = r.Create(x)
	_ = r.Create(rec(b, os.Getpid()))
	if err := r.RemovePID(777); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.Get(a); ok {
		t.Error("record of pid 777 still there")
	}
	if _, ok, _ := r.Get(b); !ok {
		t.Error("another record went too")
	}
}

func TestListOfAMissingFolderIsEmpty(t *testing.T) {
	got, err := newReg(t).List()
	if err != nil || len(got) != 0 {
		t.Errorf("%v %v", got, err)
	}
}
