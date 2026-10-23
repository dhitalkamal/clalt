package crew

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

// ErrNoLog means the account has no gateway output on disk: clalt never started
// a gateway for it.
var ErrNoLog = errors.New("there is no gateway output for this account yet")

// tailWindow is how much of the end of a log is read. a log that clalt does not
// rotate by itself can be long, and the last lines are all that is asked for.
const tailWindow = 256 << 10

// the values of a token in a url, and of a bearer header. Kiro Crew hides them
// in its own log, and this is a second look, since a token must never reach the
// screen of anyone who looks over a shoulder or reads a pasted log.
var (
	tokenValue  = regexp.MustCompile(`(?i)(token=)[^\s&"']+`)
	bearerValue = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`)
)

func hideTokens(line string) string {
	line = tokenValue.ReplaceAllString(line, "${1}<hidden>")
	return bearerValue.ReplaceAllString(line, "${1}<hidden>")
}

// Tail returns the last n lines of the file at path, in order and with token
// values hidden. n of 0 or less means every line of the end that is read. a
// file that is not there is ErrNoLog.
func Tail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoLog
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := max(0, fi.Size()-tailWindow)
	buf := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && len(buf) > 0 {
		return nil, err
	}
	text := string(buf)
	if start > 0 {
		// the window began inside a line
		_, text, _ = strings.Cut(text, "\n")
	}
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, line := range lines {
		lines[i] = hideTokens(strings.TrimSuffix(line, "\r"))
	}
	return lines, nil
}

// Logs is the end of what the account's gateway printed, see Tail.
func (m *Manager) Logs(id string, n int) ([]string, error) {
	p, err := PathsFor(id)
	if err != nil {
		return nil, err
	}
	return Tail(p.Log, n)
}
