package crew

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func sampleRecord() Record {
	return Record{
		PID: 4242, Port: 5480, Home: "/h/work/home", Version: "0.9.0",
		StartedAt: time.Date(2026, 10, 23, 15, 5, 12, 0, time.UTC), ProcStart: "Thu Oct 23 15:05:12 2026",
	}
}

func TestARecordIsWrittenAndReadBack(t *testing.T) {
	isolate(t)
	rec := sampleRecord()
	if err := WriteRecord("work", rec); err != nil {
		t.Fatal(err)
	}
	got, found, err := ReadRecord("work")
	if err != nil || !found {
		t.Fatalf("ReadRecord = %+v, %v, %v", got, found, err)
	}
	rec.Account = "work"
	if got != rec {
		t.Errorf("read back %+v, wrote %+v", got, rec)
	}
}

func TestARecordIsPrivateToTheUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix permission bits")
	}
	isolate(t)
	if err := WriteRecord("work", sampleRecord()); err != nil {
		t.Fatal(err)
	}
	p, _ := PathsFor("work")
	for path, want := range map[string]os.FileMode{p.Record: 0o600, p.Dir: 0o700, Root(): 0o700} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != want {
			t.Errorf("%s has mode %v, want %v", path, fi.Mode().Perm(), want)
		}
	}
}

func TestNoRecordIsNotAnError(t *testing.T) {
	isolate(t)
	if _, found, err := ReadRecord("work"); found || err != nil {
		t.Errorf("found=%v err=%v for an account that never started a gateway", found, err)
	}
	if err := RemoveRecord("work"); err != nil {
		t.Errorf("removing a record that is not there: %v", err)
	}
}

func TestACorruptRecordIsAnErrorThatNamesTheFile(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Record, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadRecord("work"); err == nil || !strings.Contains(err.Error(), p.Record) {
		t.Errorf("err = %v, want one naming %s", err, p.Record)
	}
}

func TestARecordWithoutAPidOrPortIsCorrupt(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{}`, `{"pid":0,"port":5480}`, `{"pid":4242,"port":0}`, `{"pid":-1,"port":5480}`} {
		if err := os.WriteFile(p.Record, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ReadRecord("work"); err == nil {
			t.Errorf("%s was accepted", body)
		}
	}
}

func TestTheAccountOfARecordIsTheFolderItIsInNotWhatItClaims(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Record, []byte(`{"account":"someone-else","pid":4242,"port":5480}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, _, err := ReadRecord("work")
	if err != nil || rec.Account != "work" {
		t.Errorf("rec = %+v, err = %v", rec, err)
	}
}

func TestRemovingARecordKeepsTheRestOfTheAccountsFolder(t *testing.T) {
	isolate(t)
	if err := WriteRecord("work", sampleRecord()); err != nil {
		t.Fatal(err)
	}
	p, _ := PathsFor("work")
	if err := os.WriteFile(p.Log, []byte("log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveRecord("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Record); !os.IsNotExist(err) {
		t.Errorf("the record is still there: %v", err)
	}
	if _, err := os.Stat(p.Log); err != nil {
		t.Errorf("the log went with it: %v", err)
	}
}

func TestListGivesEveryRecordSortedAndReportsACorruptOne(t *testing.T) {
	isolate(t)
	for _, id := range []string{"zed", "alpha"} {
		if err := WriteRecord(id, sampleRecord()); err != nil {
			t.Fatal(err)
		}
	}
	bad, _ := PathsFor("broken")
	if err := os.MkdirAll(bad.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad.Record, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	// a folder with data and no record is an account that is not running
	if err := os.MkdirAll(filepath.Join(Root(), "idle", "home"), 0o700); err != nil {
		t.Fatal(err)
	}

	entries, err := List()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.Account)
		if (e.Account == "broken") != (e.Err != nil) {
			t.Errorf("%s: err = %v", e.Account, e.Err)
		}
	}
	if got := strings.Join(ids, ","); got != "alpha,broken,zed" {
		t.Errorf("entries = %s, want alpha,broken,zed", got)
	}
}

func TestListWithNoCrewFolderIsEmpty(t *testing.T) {
	isolate(t)
	if entries, err := List(); err != nil || len(entries) != 0 {
		t.Errorf("List() = %v, %v", entries, err)
	}
}
