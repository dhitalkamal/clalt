package crew

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Proc is what ps says about one process.
type Proc struct {
	Alive bool
	// Start is the start time as ps prints it, in five words. it is kept as
	// text and only compared, so no locale or time zone can change what it means.
	Start   string
	Command string
}

// Probe asks ps about one process. a pid that ps does not know is not alive. ps
// that cannot be run is an error, because then nothing is known, and an error is
// not a process that has ended.
func Probe(ctx context.Context, r Runner, env []string, pid int) (Proc, error) {
	if pid <= 0 {
		return Proc{}, nil
	}
	spec := Spec{
		Bin:  "ps",
		Args: []string{"-ww", "-o", "lstart=,command=", "-p", strconv.Itoa(pid)},
		Env:  append(slices.Clone(env), "LC_ALL=C"),
	}
	stdout, stderr, err := r.Output(ctx, spec)
	if err != nil {
		var exited interface{ ExitCode() int }
		// ps exits 1 with nothing to say for a pid it does not know. one that
		// says something is a ps that did not understand the question
		if errors.As(err, &exited) && len(bytes.TrimSpace(stdout)) == 0 && len(bytes.TrimSpace(stderr)) == 0 {
			return Proc{}, nil
		}
		return Proc{}, fmt.Errorf("cannot ask ps about pid %d: %w", pid, err)
	}
	return parsePS(pid, string(stdout))
}

// parsePS reads "Thu Oct 23 15:05:12 2026 command and arguments". the start is
// always five words, a day with one digit included, since ps pads it with a
// space.
func parsePS(pid int, out string) (Proc, error) {
	fields := strings.Fields(firstLine(out))
	if len(fields) < 6 || !strings.Contains(fields[3], ":") || len(fields[4]) != 4 {
		return Proc{}, fmt.Errorf("ps answered about pid %d in a form clalt does not know", pid)
	}
	command := strings.Join(fields[5:], " ")
	// a process that has ended and is waiting to be collected
	if strings.Contains(command, "<defunct>") {
		return Proc{}, nil
	}
	return Proc{Alive: true, Start: strings.Join(fields[:5], " "), Command: command}, nil
}

// Judge says where a recorded gateway stands, given what ps says about its pid,
// and why when it is not running. a process is the gateway only when it has the
// start time that was kept at the launch and its command line is still a Kiro
// Crew gateway on the recorded port. a pid with another start time is another
// process whatever it is called.
func Judge(rec Record, p Proc) (State, string) {
	switch {
	case !p.Alive:
		return StateStale, fmt.Sprintf("pid %d is not running", rec.PID)
	case rec.ProcStart == "":
		return StateUnverified, fmt.Sprintf("no start time was kept for pid %d, so it cannot be told from another process", rec.PID)
	case p.Start != rec.ProcStart:
		return StateStale, fmt.Sprintf("pid %d belongs to another process now", rec.PID)
	case !looksLikeGateway(p.Command, rec.Port):
		return StateUnverified, fmt.Sprintf("pid %d started when the gateway did, but its command line is not a Kiro Crew gateway on port %d", rec.PID, rec.Port)
	}
	return StateRunning, ""
}

// looksLikeGateway is true for a command line that runs a Kiro Crew gateway on
// the port: the launcher is the program (kirocrew), or the script a shell or
// python was given, or the module kiro_crew, and the next word that is not a
// flag is gateway, followed later by --port and the port. a word that only
// appears somewhere else in the line does not count.
func looksLikeGateway(command string, port int) bool {
	argv := strings.Fields(command)
	rest := afterLauncher(argv)
	if rest == nil {
		return false
	}
	for len(rest) > 0 && strings.HasPrefix(rest[0], "-") {
		rest = rest[1:]
	}
	if len(rest) == 0 || rest[0] != "gateway" {
		return false
	}
	want := strconv.Itoa(port)
	for i, f := range rest[1:] {
		if f == "--port="+want || (f == "--port" && i+2 < len(rest) && rest[i+2] == want) {
			return true
		}
	}
	return false
}

// afterLauncher returns the arguments that follow the Kiro Crew launcher, or nil
// when the command is not one.
func afterLauncher(argv []string) []string {
	isLauncher := func(s string) bool { return strings.EqualFold(filepath.Base(s), "kirocrew") }
	switch {
	case len(argv) == 0:
		return nil
	case isLauncher(argv[0]):
		return argv[1:]
	case len(argv) > 1 && isInterpreter(argv[0]) && isLauncher(argv[1]):
		return argv[2:]
	}
	if isInterpreter(argv[0]) {
		for i := 1; i+1 < len(argv); i++ {
			if argv[i] == "-m" && argv[i+1] == "kiro_crew" {
				return argv[i+2:]
			}
			if !strings.HasPrefix(argv[i], "-") {
				break
			}
		}
	}
	return nil
}

// isInterpreter is a shell or python, the programs a launcher runs under.
func isInterpreter(s string) bool {
	b := strings.ToLower(filepath.Base(s))
	return b == "sh" || b == "bash" || strings.HasPrefix(b, "python")
}
