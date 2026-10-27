//go:build !windows

package officed

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// helper mode: be a stand in server that says hello and waits
	if os.Getenv("OFFICED_HELPER") == "1" {
		buf := make([]byte, 1)
		n, _ := os.Stdin.Read(buf)
		os.Stdout.WriteString("hello stdin-eof=" + map[bool]string{true: "yes", false: "no"}[n == 0] + "\n")
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestSpawnDetachesIntoItsOwnSessionWithLogAndNoStdin(t *testing.T) {
	t.Setenv("OFFICED_HELPER", "1")
	log := filepath.Join(t.TempDir(), "office.log")
	exe, _ := os.Executable()
	child, err := Spawn(exe, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(child.PID, syscall.SIGKILL); <-child.Exited })
	sid, err := syscall.Getpgid(child.PID)
	if err != nil || sid != child.PID {
		t.Errorf("sid %d pid %d err %v: not a group leader (Setsid makes it one)", sid, child.PID, err)
	}
	if mine, _ := syscall.Getpgid(0); sid == mine {
		t.Error("child shares our process group")
	}
	var got string
	for i := 0; i < 100 && !strings.Contains(got, "hello"); i++ {
		raw, _ := os.ReadFile(log)
		got = string(raw)
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(got, "hello stdin-eof=yes") {
		t.Errorf("log %q", got)
	}
	if fi, _ := os.Stat(log); fi.Mode().Perm() != 0o600 {
		t.Errorf("log mode %v", fi.Mode())
	}
}

func TestAliveAndStop(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Error("self not alive")
	}
	if Alive(0) || Alive(-3) {
		t.Error("bad pids alive")
	}
}
