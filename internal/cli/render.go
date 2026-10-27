package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// out is the app's output as a writer that remembers the first error of a write,
// for everything that reports. a report that could not be written was lost, and
// the command has to end by saying so. what is handed to claude as its terminal
// is a.Out itself, not this.
func (a *App) out() io.Writer { return recordingWriter{a} }

type recordingWriter struct{ a *App }

func (w recordingWriter) Write(p []byte) (int, error) {
	n, err := w.a.Out.Write(p)
	if err != nil && w.a.writeErr == nil {
		w.a.writeErr = err
	}
	return n, err
}

// finish is what a command ends with: its own error, or when it has none and a
// write failed, the failed write. a bare exit code (doctor with a failed check,
// a session that ended) has no message of its own, so the lost output is said
// beside it and the code stays.
func (a *App) finish(err error) error {
	if a.writeErr == nil {
		return err
	}
	lost := fmt.Errorf("the output could not be written: %w", a.writeErr)
	if err == nil || errors.Is(err, a.writeErr) || err.Error() == a.writeErr.Error() {
		return lost
	}
	var exit exitError
	if errors.As(err, &exit) {
		fmt.Fprintf(a.Err, "error: %s\n", clean(lost.Error()))
	}
	return err
}

// emitJSON writes v as indented JSON to the app's output.
func (a *App) emitJSON(v any) error {
	enc := json.NewEncoder(a.out())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printf writes a formatted human line to the app's output.
func (a *App) printf(format string, args ...any) {
	fmt.Fprintf(a.out(), format, args...)
}

// prompt reads one trimmed line after showing a message. It reuses a single
// buffered reader so read-ahead does not swallow input meant for a later prompt.
func (a *App) prompt(msg string) (string, error) {
	fmt.Fprint(a.out(), msg)
	if a.reader == nil {
		a.reader = bufio.NewReader(a.In)
	}
	line, err := a.reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// confirm asks a yes/no question, defaulting to no. an answer that is not given
// at all, because the input ended, is an error and not a no: a script has to be
// told that nobody was asked.
func (a *App) confirm(msg string) (bool, error) {
	ans, err := a.prompt(msg + " [y/N]: ")
	if err != nil {
		return false, err
	}
	ans = strings.ToLower(ans)
	return ans == "y" || ans == "yes", nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
