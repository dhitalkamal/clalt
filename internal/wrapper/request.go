package wrapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// requestVersion is the version of the request file. a request of another
// version is refused, and then removed, so an old wrapper never acts on a format
// it does not know.
const requestVersion = 1

// Request asks a running session to move to another account. it is a file next
// to the session record, <handle>.move, which the wrapper polls: that works
// where unix sockets do not. the name does not end in .json on purpose, so a
// registry that lists *.json and removes what it cannot read never meets it.
type Request struct {
	Version     int       `json:"version"`
	To          string    `json:"to"` // account id
	RequestedAt time.Time `json:"requestedAt"`
}

// ValidHandle says whether h is a session handle: 16 lower case hex characters.
// a handle becomes part of a file name, so nothing else is let through.
func ValidHandle(h string) bool {
	if len(h) != 16 {
		return false
	}
	for _, c := range h {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// RequestPath is where the request for a session is kept.
func RequestPath(dir, handle string) (string, error) {
	if !ValidHandle(handle) {
		return "", fmt.Errorf("%q is not a session handle", handle)
	}
	return filepath.Join(dir, handle+".move"), nil
}

// WriteRequest asks for a move. it replaces a request that was not taken yet, and
// is written in one piece so the wrapper never reads half of it.
func WriteRequest(dir, handle string, r Request) error {
	path, err := RequestPath(dir, handle)
	if err != nil {
		return err
	}
	if r.To == "" {
		return errors.New("a move request needs an account to move to")
	}
	r.Version = requestVersion
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o600)
}

// HasRequest says whether a request is waiting for the session.
func HasRequest(dir, handle string) bool {
	path, err := RequestPath(dir, handle)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// ClaimRequest takes the request of the session, if there is one. it is renamed
// before it is read, so a newer request written while the move goes on is a
// request of its own for the next pause. a request that cannot be used is an
// error and is gone all the same, or it would be tried again at every poll.
func ClaimRequest(dir, handle string) (Request, bool, error) {
	path, err := RequestPath(dir, handle)
	if err != nil {
		return Request{}, false, err
	}
	claimed := path + ".claimed"
	if err := os.Rename(path, claimed); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Request{}, false, nil
		}
		return Request{}, false, err
	}
	raw, readErr := os.ReadFile(claimed)
	_ = os.Remove(claimed)
	if readErr != nil {
		return Request{}, false, readErr
	}
	var r Request
	switch err := json.Unmarshal(raw, &r); {
	case err != nil:
		return Request{}, false, fmt.Errorf("the move request of %s is not readable: %w", handle, err)
	case r.Version != requestVersion:
		return Request{}, false, fmt.Errorf("the move request of %s has version %d, this clalt reads version %d", handle, r.Version, requestVersion)
	case r.To == "":
		return Request{}, false, fmt.Errorf("the move request of %s names no account", handle)
	}
	return r, true, nil
}

// RemoveRequests clears what is left of the requests of a session, when it ends.
func RemoveRequests(dir, handle string) {
	path, err := RequestPath(dir, handle)
	if err != nil {
		return
	}
	_ = os.Remove(path)
	_ = os.Remove(path + ".claimed")
	_ = os.Remove(path + ".result")
	if stop, err := stopPath(dir, handle); err == nil {
		_ = os.Remove(stop)
	}
}

// Result is why a request to move was not carried out.
type Result struct {
	To    string    `json:"to"`
	Error string    `json:"error"`
	At    time.Time `json:"at"`
}

// WriteResult leaves the reason a request was not carried out next to the
// session record, as <handle>.move.result, for `clalt move` to say the next time.
func WriteResult(dir, handle string, r Result) error {
	path, err := RequestPath(dir, handle)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path+".result", raw, 0o600)
}

// TakeResult reads the result of the last failed request and removes it.
func TakeResult(dir, handle string) (Result, bool) {
	path, err := RequestPath(dir, handle)
	if err != nil {
		return Result{}, false
	}
	raw, err := fsutil.ReadFile(path + ".result")
	if err != nil {
		return Result{}, false
	}
	_ = os.Remove(path + ".result")
	var r Result
	if json.Unmarshal(raw, &r) != nil {
		return Result{}, false
	}
	return r, true
}
