package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/fakes/fakeutil"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// fakeMain is what this binary does when it is started as claude. it records
// its arguments and environment next to itself, appends every call to a calls
// file, and answers from files the test drops there: <name>.out is printed with
// @CONFIG_DIR@ replaced by CLAUDE_CONFIG_DIR, <name>.err goes to stderr and
// <name>.code is the exit code, where name is status, login, logout or version.
// a plain --version prints 2.1.303 unless version.out says otherwise. <name>.die
// names a signal it sends itself. a hang file makes every call except
// --version wait for an interrupt instead, in the mode the file names, and
// <name>.hang does that for one command only. the environment of the last call
// is in env, and that of the last call of each command in env.<name>. the
// process group of the last call is in pgid.<name>. the hang mode settle is for
// a call that has to be left alone: it notes an interrupt, and otherwise ends by
// itself after a second and writes finished.
func fakeMain(args []string) int {
	dir := fakeutil.Beside()
	at := func(name string) string { return filepath.Join(dir, name) }
	fakeutil.AppendLine(at("calls"), strings.Join(args, " "))
	argv := "\n"
	if len(args) > 0 {
		argv = strings.Join(args, "\n") + "\n"
	}
	_ = os.WriteFile(at("argv"), []byte(argv), 0o644)
	fakeutil.WriteEnv(at("env"))

	name := "version"
	if len(args) >= 2 && args[0] == "auth" {
		switch args[1] {
		case "status", "login", "logout":
			name = args[1]
		}
	}
	fakeutil.WriteEnv(at("env." + name))
	fakeutil.NotePgid(at("pgid." + name))

	hangfile := ""
	if _, err := os.Stat(at("hang")); err == nil && name != "version" {
		hangfile = at("hang")
	}
	if _, err := os.Stat(at(name + ".hang")); err == nil {
		hangfile = at(name + ".hang")
	}
	if hangfile != "" {
		hangUntilInterrupted(dir, hangfile)
		return 0
	}

	if b, err := fsutil.ReadFile(at(name + ".err")); err == nil {
		os.Stderr.Write(b)
	}
	if b, err := fsutil.ReadFile(at(name + ".die")); err == nil {
		fakeutil.KillSelf(strings.TrimSpace(string(b)))
	}
	_, hasOut := fileExists(at("version.out"))
	if name == "version" && !hasOut {
		fmt.Println("2.1.303 (Claude Code)")
		return 0
	}
	if b, err := fsutil.ReadFile(at(name + ".out")); err == nil {
		// the output is JSON, where a windows path needs its backslashes doubled
		os.Stdout.WriteString(strings.ReplaceAll(string(b), "@CONFIG_DIR@", strings.ReplaceAll(os.Getenv("CLAUDE_CONFIG_DIR"), `\`, `\\`)))
	}
	if b, err := fsutil.ReadFile(at(name + ".code")); err == nil {
		n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		return n
	}
	return 0
}

func fileExists(path string) (os.FileInfo, bool) {
	fi, err := os.Stat(path)
	return fi, err == nil
}

// hangUntilInterrupted waits as the mode in hangfile says: ignore keeps going
// through an interrupt, slow takes a second to finish after one, settle ends by
// itself after a second, anything else stops at once on an interrupt.
func hangUntilInterrupted(dir, hangfile string) {
	at := func(name string) string { return filepath.Join(dir, name) }
	raw, _ := fsutil.ReadFile(hangfile)
	mode := strings.TrimSpace(string(raw))
	delay := 5 * time.Second
	if mode == "settle" {
		delay = time.Second
	}
	ch := make(chan os.Signal, 1)
	if mode == "ignore" {
		signal.Ignore(os.Interrupt)
	} else {
		signal.Notify(ch, os.Interrupt)
	}
	_ = os.WriteFile(at("ready"), []byte("ready\n"), 0o644)
	select {
	case <-ch:
		_ = os.WriteFile(at("interrupted"), []byte("interrupted\n"), 0o644)
		if mode == "slow" {
			time.Sleep(time.Second)
			_ = os.WriteFile(at("finished"), []byte("finished\n"), 0o644)
			os.Exit(0)
		}
		os.Exit(130)
	case <-time.After(delay):
		if mode == "settle" {
			_ = os.WriteFile(at("finished"), []byte("finished\n"), 0o644)
		}
	}
}

func main() {
	switch fakeutil.Program() {
	case "claude-guard":
		os.Exit(fakeutil.Guard())
	case "claude-exit":
		// ends the way the arguments say: "exit N" or "kill9"
		if len(os.Args) > 2 && os.Args[1] == "exit" {
			n, _ := strconv.Atoi(os.Args[2])
			os.Exit(n)
		}
		fakeutil.KillSelf("KILL")
		os.Exit(1)
	}
	os.Exit(fakeMain(os.Args[1:]))
}
