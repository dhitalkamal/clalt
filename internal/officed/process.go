package officed

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"
)

// Probe is the handshake: GET /api/ping on the run file's port with its token
// must answer as a clalt office server with the run file's pid. a pid that
// was reused, a port taken by something else or a wrong token all fail here.
func Probe(r Run) error {
	req, err := http.NewRequest(http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", strconv.Itoa(r.Port))+"/api/ping", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Clalt-Token", r.Token)
	client := http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping answered %d", resp.StatusCode)
	}
	var reply struct {
		App string `json:"app"`
		PID int    `json:"pid"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&reply); err != nil {
		return err
	}
	if reply.App != "clalt-office" || reply.PID != r.PID {
		return errors.New("not the clalt office server of that pid")
	}
	return nil
}

// Alive says whether a process with this pid exists. no permission to signal
// it still means it exists.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Terminate asks the process to end (SIGTERM). callers verify the pid first.
func Terminate(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
