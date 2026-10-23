package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SessionAccount says that a claude session ran under an account from Ts on.
// clalt's hooks write one when they first see a claude session id under a
// clalt run session, and again when the id shows up under another account (a
// conversation that was moved).
type SessionAccount struct {
	Ts        time.Time `json:"ts"`
	SessionID string    `json:"sessionId"`
	Account   string    `json:"account"`
}

// AttributeSessions is AttributeUntil for the time after the switch log
// stopped: records before until go by the log, records from until on go to the
// account their claude session ran under, and to Unattributed when no session
// of clalt run saw that id. a session that moved is split by time: a record
// belongs to the latest mapping at or before it, or to the first one when it
// predates them all (the session started under that account).
func AttributeSessions(records []Record, events []SwitchEvent, sessions []SessionAccount, until time.Time) map[string][]Record {
	if until.IsZero() {
		return AttributeUntil(records, events, until)
	}
	var before []Record
	byID := map[string][]SessionAccount{}
	for _, s := range sessions {
		byID[s.SessionID] = append(byID[s.SessionID], s)
	}
	for _, list := range byID {
		sort.Slice(list, func(i, j int) bool { return list[i].Ts.Before(list[j].Ts) })
	}
	out := map[string][]Record{}
	for _, r := range records {
		if r.Timestamp.Before(until) {
			before = append(before, r)
			continue
		}
		account := Unattributed
		if list := byID[r.SessionID]; r.SessionID != "" && len(list) > 0 {
			account = list[0].Account
			for _, m := range list {
				if m.Ts.After(r.Timestamp) {
					break
				}
				account = m.Account
			}
		}
		out[account] = append(out[account], r)
	}
	for account, recs := range AttributeUntil(before, events, until) {
		out[account] = append(out[account], recs...)
	}
	return out
}

// ReadSessionAccounts reads the mapping file. a missing file is no mapping, and
// a line that is not one is skipped, so a half written line costs only itself.
func ReadSessionAccounts(path string) ([]SessionAccount, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []SessionAccount
	r := bufio.NewReader(f)
	skipping := false
	for {
		line, prefix, err := r.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
		// a line longer than the reader's buffer is dropped whole, not fatal
		if prefix {
			skipping = true
			continue
		}
		if skipping {
			skipping = false
			continue
		}
		var m SessionAccount
		if json.Unmarshal(line, &m) == nil && m.SessionID != "" && m.Account != "" {
			out = append(out, m)
		}
	}
}

// AppendSessionAccount adds one mapping. one short line in one write, so
// parallel sessions do not tear each other's lines.
func AppendSessionAccount(path string, m SessionAccount) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(m)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}
