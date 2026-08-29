package account

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SwitchEvent records that an account became active at a point in time. The log
// lets the usage dashboard attribute local session activity to the account that
// was active when it happened.
type SwitchEvent struct {
	Ts      time.Time `json:"ts"`
	Account string    `json:"account"`
}

// appendSwitchLog appends one event as a JSONL line.
func (s *Store) appendSwitchLog(name string) error {
	if err := os.MkdirAll(filepath.Dir(s.logPath), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(SwitchEvent{Ts: s.now(), Account: name})
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// ReadSwitchLog returns all recorded switch events in chronological order.
func (s *Store) ReadSwitchLog() ([]SwitchEvent, error) {
	raw, err := os.ReadFile(s.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var events []SwitchEvent
	for _, line := range splitLines(raw) {
		if len(line) == 0 {
			continue
		}
		var e SwitchEvent
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines rather than fail the whole read
		}
		events = append(events, e)
	}
	return events, nil
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}
