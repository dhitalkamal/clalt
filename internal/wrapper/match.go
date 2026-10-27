package wrapper

import (
	"fmt"
	"strconv"
	"strings"
)

// minIDPrefix is the shortest claude session id prefix that is looked up: with
// fewer characters it would name too many sessions to be a choice.
const minIDPrefix = 4

// Match finds the running session that ref names: its handle, the pid of its
// wrapper, or the start of its claude session id, in that order. a prefix that
// fits several sessions is an error that names them.
func Match(records []Record, ref string) (Record, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Record{}, fmt.Errorf("which session? give its handle, pid or session id")
	}
	for _, r := range records {
		if r.Handle == ref {
			return r, nil
		}
	}
	if pid, err := strconv.Atoi(ref); err == nil && pid > 0 {
		for _, r := range records {
			if r.WrapperPID == pid {
				return r, nil
			}
		}
	}
	if len(ref) < minIDPrefix {
		return Record{}, fmt.Errorf("%q is too short to be the start of a session id (at least %d characters)", ref, minIDPrefix)
	}
	var found []Record
	for _, r := range records {
		if r.ClaudeSessionID != "" && strings.HasPrefix(strings.ToLower(r.ClaudeSessionID), strings.ToLower(ref)) {
			found = append(found, r)
		}
	}
	switch len(found) {
	case 0:
		return Record{}, fmt.Errorf("no running session matches %q", ref)
	case 1:
		return found[0], nil
	}
	handles := make([]string, len(found))
	for i, r := range found {
		handles[i] = r.Handle
	}
	return Record{}, fmt.Errorf("%q fits several sessions (%s): give more of the id, or a handle", ref, strings.Join(handles, ", "))
}
