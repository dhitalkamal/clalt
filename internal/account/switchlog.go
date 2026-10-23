package account

import (
	"encoding/json"
	"os"
	"time"
)

// SwitchEvent records that an account became active at a point in time. The log
// lets usage be attributed to the account that was active when it happened.
// Only clalt v0.1.0 and earlier wrote entries: every account is a folder of its
// own now, none is active, and nothing appends to the log. It is still read, for
// the usage from before that.
type SwitchEvent struct {
	Ts      time.Time `json:"ts"`
	Account string    `json:"account"`
}

// ReadSwitchLog returns all recorded switch events in chronological order.
func (s *Store) ReadSwitchLog() ([]SwitchEvent, error) {
	if err := checkPath(s.logPath); err != nil {
		return nil, err
	}
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
