package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"sort"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// SourceDefault marks a ledger line that clalt wrote because plain claude, in
// the default home, was signed in as the account. it can be shown and undone.
const SourceDefault = "default"

// DefaultLines are the ledger lines that give the sessions clalt never saw to
// account: sessions that started at or after the cutover whose id has no line in
// ledger yet. a session that has a line is never reassigned. each line starts at
// the first record of its session, so the whole session falls under it. records
// without a session id have nothing to key a line on and stay unattributed.
func DefaultLines(records []Record, ledger []SessionAccount, cutover time.Time, account string) []SessionAccount {
	if cutover.IsZero() || account == "" {
		return nil
	}
	known := map[string]bool{}
	for _, l := range ledger {
		known[l.SessionID] = true
	}
	first := map[string]time.Time{}
	for _, r := range records {
		if r.SessionID == "" || known[r.SessionID] {
			continue
		}
		if t, ok := first[r.SessionID]; !ok || r.Timestamp.Before(t) {
			first[r.SessionID] = r.Timestamp
		}
	}
	out := make([]SessionAccount, 0, len(first))
	for id, ts := range first {
		// a session that started before the cutover began under some other
		// login, so nothing says it is this account's
		if ts.Before(cutover) {
			continue
		}
		out = append(out, SessionAccount{Ts: ts, SessionID: id, Account: account, Source: SourceDefault})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionID < out[j].SessionID })
	return out
}

// RemoveSource drops the lines of the ledger with the given source and says how
// many there were. lines that are not ours to read are kept as they are. a
// missing ledger has nothing to remove.
func RemoveSource(path, source string) (int, error) {
	raw, err := fsutil.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var kept bytes.Buffer
	removed := 0
	for _, line := range bytes.SplitAfter(raw, []byte("\n")) {
		var m SessionAccount
		if len(line) > 0 && json.Unmarshal(line, &m) == nil && m.Source == source {
			removed++
			continue
		}
		kept.Write(line)
	}
	if removed == 0 {
		return 0, nil
	}
	return removed, fsutil.WriteFileAtomic(path, kept.Bytes(), 0o600)
}
