package feed

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

const (
	// a reading with the same values as the last line is not written again
	// until this long after that line
	readingGap = 5 * time.Minute
	// lines older than this are dropped by a rotation
	readingKeepDays = 90
	// a rotation runs once the first line is this many days old, so at most
	// once a day and never on the common path
	readingRotateDays = readingKeepDays + 1
	// how much of either end of the file is read to find the first and last line
	readingEdge = 1024
)

// Reading is one line of feed/readings/<account id>.jsonl: the two limit
// windows as the feed merged them at time T. a window with no reading is null.
// nothing else is stored, no prompts and no paths.
type Reading struct {
	T                time.Time  `json:"t"`
	FiveHour         *float64   `json:"fiveHour"`
	FiveHourResetsAt *time.Time `json:"fiveHourResetsAt"`
	SevenDay         *float64   `json:"sevenDay"`
	SevenDayResetsAt *time.Time `json:"sevenDayResetsAt"`
}

func (s Store) readingsPath(account string) string {
	return filepath.Join(s.Dir, "readings", account+".jsonl")
}

func windowReading(w *Window) (*float64, *time.Time) {
	if w == nil {
		return nil, nil
	}
	pct, at := w.UsedPct, w.ResetsAt.UTC()
	return &pct, &at
}

func sameFloat(a, b *float64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func sameTime(a, b *time.Time) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b))
}

// sameValues says whether two readings hold the same numbers, whatever T is.
func (r Reading) sameValues(o Reading) bool {
	return sameFloat(r.FiveHour, o.FiveHour) && sameTime(r.FiveHourResetsAt, o.FiveHourResetsAt) &&
		sameFloat(r.SevenDay, o.SevenDay) && sameTime(r.SevenDayResetsAt, o.SevenDayResetsAt)
}

// recordReading logs the merged limits of an account. the caller holds the
// store lock. the log is secondary: a failure here must not fail the update.
func (s Store) recordReading(account string, cur AccountLimits, now time.Time) {
	if cur.FiveHour == nil && cur.SevenDay == nil {
		return
	}
	r := Reading{T: now.UTC()}
	r.FiveHour, r.FiveHourResetsAt = windowReading(cur.FiveHour)
	r.SevenDay, r.SevenDayResetsAt = windowReading(cur.SevenDay)
	_ = s.appendLine(account, r)
}

// appendLine adds r as one line. it is one write on an O_APPEND file, so lines
// of writers that do not share the lock still never interleave. unless r
// differs from the last line or the last line is old, it writes nothing. an
// old first line triggers a rotation.
func (s Store) appendLine(account string, r Reading) error {
	path := s.readingsPath(account)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if last, ok := edgeLine(f, size, true); ok && r.sameValues(last) && r.T.Sub(last.T) < readingGap && !r.T.Before(last.T) {
		return nil
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	// a crash can leave a torn last line with no newline: start our own line
	if size > 0 {
		var b [1]byte
		if _, err := f.ReadAt(b[:], size-1); err == nil && b[0] != '\n' {
			line = append([]byte{'\n'}, line...)
		}
	}
	if _, err := f.Write(line); err != nil {
		return err
	}
	if first, ok := edgeLine(f, size, false); ok && r.T.Sub(first.T) > readingRotateDays*24*time.Hour {
		_ = f.Close()
		return s.rotateReadings(account, r.T.AddDate(0, 0, -readingKeepDays))
	}
	return nil
}

// edgeLine parses the last (or first) complete line of the file from at most
// readingEdge bytes. a missing, torn or oversized line is "not there".
func edgeLine(f *os.File, size int64, last bool) (Reading, bool) {
	if size == 0 {
		return Reading{}, false
	}
	n := min(size, readingEdge)
	off := int64(0)
	if last {
		off = size - n
	}
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, off); err != nil && !errors.Is(err, io.EOF) {
		return Reading{}, false
	}
	var line []byte
	if last {
		buf = bytes.TrimSuffix(buf, []byte("\n"))
		i := bytes.LastIndexByte(buf, '\n')
		if i < 0 && off > 0 {
			return Reading{}, false
		}
		line = buf[i+1:]
	} else {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			return Reading{}, false
		}
		line = buf[:i]
	}
	var r Reading
	if json.Unmarshal(line, &r) != nil || r.T.IsZero() {
		return Reading{}, false
	}
	return r, true
}

// rotateReadings rewrites the file without the lines before cut. only the
// first append past the cut by a day gets here.
func (s Store) rotateReadings(account string, cut time.Time) error {
	all, err := s.Readings(account, cut)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	for _, r := range all {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	return fsutil.WriteFileAtomicNoSync(s.readingsPath(account), out.Bytes(), 0o600)
}

// Readings reads the log of an account from since on, oldest first. lines
// that do not parse are skipped. no file is no readings.
func (s Store) Readings(account string, since time.Time) ([]Reading, error) {
	if err := config.CheckAccountID(account); err != nil {
		return nil, err
	}
	f, err := os.Open(s.readingsPath(account))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Reading
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadBytes('\n')
		var r Reading
		if len(line) > 0 && json.Unmarshal(line, &r) == nil && !r.T.IsZero() && !r.T.Before(since) {
			out = append(out, r)
		}
		if err != nil {
			return out, nil
		}
	}
}
