// Package fakeutil holds what the test fakes of claude and kirocrew share. the
// fakes are small programs the tests build once and run in place of the real
// ones, so no test needs a shell script and the same fakes run on every
// platform.
package fakeutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Beside is the folder the fake lives in, where it keeps its records.
func Beside() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return filepath.Dir(os.Args[0])
}

// Program is the name the fake was started under, without .exe: a test can
// install the same binary as claude or as a guard that refuses.
func Program() string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
}

// Guard ends the process the way the guard does: it says a test reached claude
// without a fake, and exits 99.
func Guard() int {
	fmt.Fprintln(os.Stderr, "a test ran claude without installing a fake")
	return 99
}

// AppendLine adds one line to a file.
func AppendLine(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		fmt.Fprintln(f, line)
		f.Close()
	}
}

// WriteFile replaces a file, ignoring errors: a record that cannot be written
// shows up as a test that finds nothing.
func WriteFile(path, body string) { _ = os.WriteFile(path, []byte(body), 0o644) }

// WriteEnv records the environment sorted, one NAME=value per line.
func WriteEnv(path string) {
	env := os.Environ()
	sort.Strings(env)
	WriteFile(path, strings.Join(env, "\n")+"\n")
}
