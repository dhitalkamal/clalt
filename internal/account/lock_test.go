package account

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// parallel `clalt run` sessions each hold their own Store on one state.json.
func TestParallelStoresLoseNoUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	const n = 32
	setup := New(path, "")
	for i := 0; i < n; i++ {
		if _, err := setup.Create(fmt.Sprintf("a%02d", i), noFolders); err != nil {
			t.Fatal(err)
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := New(path, "")
			<-start
			if err := s.SetBudget(fmt.Sprintf("a%02d", i), float64(i+1), -1); err != nil {
				t.Errorf("SetBudget: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	list, err := New(path, "").List()
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range list {
		if a.Meta.DailyBudget != float64(i+1) {
			t.Errorf("%s: budget %v, want %d (an update was lost)", a.Name, a.Meta.DailyBudget, i+1)
		}
	}
}

// a reader must see the old file or the new one, never a half-written one.
func TestReadersNeverSeeATornStateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	const n = 40
	setup := New(path, "")
	for i := 0; i < n; i++ {
		if _, err := setup.Create(fmt.Sprintf("a%02d", i), noFolders); err != nil {
			t.Fatal(err)
		}
	}

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for r := 0; r < 4; r++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			s := New(path, "")
			for {
				select {
				case <-stop:
					return
				default:
				}
				list, err := s.List()
				if err != nil || len(list) != n {
					t.Errorf("reader saw %d accounts, err %v", len(list), err)
					return
				}
			}
		}()
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for i := 0; time.Now().Before(deadline); i++ {
		if err := setup.SetBudget(fmt.Sprintf("a%02d", i%n), float64(i), -1); err != nil {
			// not Fatal: the readers have to be stopped and waited for first
			t.Error(err)
			break
		}
	}
	close(stop)
	readers.Wait()
}

func TestStateFileIsPrivateAndLeavesNoTempFiles(t *testing.T) {
	s := newStore(t)
	mustCreate(t, s, "a")
	if err := s.SetBudget("a", 5, 5); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(s.statePath)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no permission bits to speak of
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("state.json mode = %v, want 0600", fi.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(s.statePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("left a temp file behind: %s", e.Name())
		}
	}
}

func TestEveryWriteGoesThroughTheLock(t *testing.T) {
	s := newStore(t)
	mustCreate(t, s, "a")
	if _, err := os.Stat(s.statePath + ".lock"); err != nil {
		t.Errorf("no lock file next to state.json: %v", err)
	}
}
