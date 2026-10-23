package crew

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// Record is what clalt keeps about a gateway it started. it is written right
// after the launch and removed when the gateway is stopped, so a record means
// that clalt started something and has not seen it end.
type Record struct {
	// Account is the account id. it is the folder the record is in, whatever the
	// file says.
	Account string `json:"account"`
	PID     int    `json:"pid"`
	Port    int    `json:"port"`
	Home    string `json:"home"`
	Version string `json:"version"`
	// StartedAt is clalt's own clock at the launch, for the uptime.
	StartedAt time.Time `json:"startedAt"`
	// ProcStart is what ps printed as the start time of the pid right after the
	// launch. it is only compared with what ps says later, never parsed: the same
	// pid with another start time is another process.
	ProcStart string `json:"procStart"`
}

// Entry is one gateway folder as List found it: its record, or why that could
// not be read.
type Entry struct {
	Account string
	Record  Record
	Err     error
}

// ReadRecord reads the record of an account. an account that has none is not an
// error: found is false.
func ReadRecord(id string) (rec Record, found bool, err error) {
	p, err := PathsFor(id)
	if err != nil {
		return Record{}, false, err
	}
	raw, err := os.ReadFile(p.Record)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Record{}, false, nil
	case err != nil:
		return Record{}, false, err
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return Record{}, false, fmt.Errorf("the gateway record %s is not readable: %w", p.Record, err)
	}
	if rec.PID <= 0 || rec.Port <= 0 || rec.Port > 65535 {
		return Record{}, false, fmt.Errorf("the gateway record %s has no usable pid and port", p.Record)
	}
	rec.Account = id
	return rec, true, nil
}

// WriteRecord saves the record of an account, atomically and for its owner only.
func WriteRecord(id string, rec Record) error {
	p, err := PathsFor(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		return err
	}
	rec.Account = id
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p.Record, append(raw, '\n'), 0o600)
}

// RemoveRecord deletes the record of an account and nothing else in its folder.
func RemoveRecord(id string) error {
	p, err := PathsFor(id)
	if err != nil {
		return err
	}
	if err := os.Remove(p.Record); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// List finds every gateway record, sorted by account id. a record that cannot be
// read is an entry with its error, so one bad file does not hide the others.
func List() ([]Entry, error) {
	dirs, err := os.ReadDir(Root())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		rec, found, err := ReadRecord(d.Name())
		switch {
		case err != nil:
			out = append(out, Entry{Account: d.Name(), Err: err})
		case found:
			out = append(out, Entry{Account: d.Name(), Record: rec})
		}
	}
	return out, nil
}
