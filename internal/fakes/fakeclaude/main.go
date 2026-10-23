package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dhitalkamal/clalt/internal/fakes/fakeutil"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// fakeClaudeMain is what the claude fake does when this binary is started as
// claude. a login is a file inside CLAUDE_CONFIG_DIR, which is how the real one
// keys it to the folder, and the FAKE_CLAUDE_* variables steer what it does. it
// records what it was asked next to itself: one line per auth call in calls, and
// for a session the arguments, the environment and the folder it ran in.
// FAKE_CLAUDE_SESSION_HOLDS makes a session wait until a session.release file
// appears, and note in signals every TERM, HUP or INT it gets.
func fakeClaudeMain(args []string, stdin io.Reader) int {
	dir := fakeutil.Beside()
	cfg := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfg == "" {
		// like the real one: no variable means the default home. tests set HOME
		// on every platform, windows included
		h := os.Getenv("HOME")
		if h == "" {
			h, _ = os.UserHomeDir()
		}
		cfg = filepath.Join(h, ".claude")
	}
	login := filepath.Join(cfg, ".fake-login")
	on := func(name string) bool { return os.Getenv("FAKE_CLAUDE_"+name) != "" }
	or := func(name, def string) string {
		if v := os.Getenv("FAKE_CLAUDE_" + name); v != "" {
			return v
		}
		return def
	}
	say := func(w io.Writer, s string) { fmt.Fprintln(w, s) }

	if len(args) >= 2 && args[0] == "auth" {
		switch args[1] {
		case "login":
			fakeutil.AppendLine(filepath.Join(dir, "calls"), "login "+cfg+"|"+strings.Join(args, " "))
			fakeutil.WriteEnv(filepath.Join(dir, "login.env"))
			if on("READS_STDIN") {
				line, _ := bufio.NewReader(stdin).ReadString('\n')
				fakeutil.WriteFile(filepath.Join(dir, "login.stdin"), strings.TrimSuffix(line, "\n")+"\n")
			}
			if on("LOGIN_FAILS") {
				say(os.Stderr, "login was cancelled")
				return 1
			}
			if on("LOGIN_NOOP") {
				return 0
			}
			if on("LOGIN_WRITES_THEN_FAILS") {
				saveLogin(cfg, login, or("EMAIL", ""), or("ORG", "Example Org"), or("PLAN", "max"))
				say(os.Stderr, "could not save the profile: network error")
				return 1
			}
			org := or("ORG", "Example Org")
			if on("NO_ORG") {
				org = ""
			}
			saveLogin(cfg, login, os.Getenv("FAKE_CLAUDE_EMAIL"), org, or("PLAN", "max"))
			say(os.Stdout, "Login successful")
			return 0
		case "status":
			return fakeStatus(dir, cfg, login, on, or)
		case "logout":
			fakeutil.AppendLine(filepath.Join(dir, "calls"), "logout "+cfg)
			fakeutil.WriteEnv(filepath.Join(dir, "logout.env"))
			if on("LOGOUT_HANGS") {
				time.Sleep(10 * time.Second)
			}
			if on("LOGOUT_FAILS") {
				say(os.Stderr, percentB(or("LOGOUT_ERR", "could not reach the keychain")))
				return 1
			}
			_ = os.Remove(login)
			return 0
		}
	}
	if len(args) >= 1 && args[0] == "--version" {
		say(os.Stdout, or("VERSION", "2.1.303")+" (Claude Code)")
		return 0
	}
	return fakeSession(dir, args, stdin, on)
}

func fakeStatus(dir, cfg, login string, on func(string) bool, or func(string, string) string) int {
	fakeutil.AppendLine(filepath.Join(dir, "calls"), "status "+cfg)
	fakeutil.WriteEnv(filepath.Join(dir, "status.env"))
	if warn, err := fsutil.ReadFile(filepath.Join(cfg, ".fake-warn")); err == nil {
		os.Stderr.Write(warn)
	}
	if on("STATUS_HANGS") {
		time.Sleep(10 * time.Second)
	}
	if on("STATUS_BROKEN") {
		fmt.Fprintln(os.Stderr, percentB(or("STATUS_ERR", "keychain is locked")))
		return 3
	}
	q := rawQuote
	// a shell printf puts these in as they are, and the tests plant JSON escapes
	// and raw control characters in them on purpose
	r := func(s string) string { return `"` + s + `"` }
	if on("API_KEY_HELPER") {
		fmt.Printf("{\"loggedIn\":true,\"authMethod\":\"api_key_helper\",\"configDirectory\":%s}\n", q(cfg))
		return 0
	}
	if raw, err := fsutil.ReadFile(login); err == nil {
		f := append(strings.Split(string(raw), "\n"), "", "", "")
		fmt.Printf("{\"loggedIn\":true,\"authMethod\":\"claude.ai\",\"apiProvider\":%s,\"email\":%s,\"orgId\":\"org-1\","+
			"\"orgName\":%s,\"subscriptionType\":%s,\"configDirectory\":%s,\"projectsDirectory\":%s}\n",
			r(or("PROVIDER", "firstParty")), r(f[0]), r(f[1]), r(f[2]), q(or("STATUS_FOLDER", cfg)), q(cfg+"/projects"))
		return 0
	}
	fmt.Printf("{\"loggedIn\":false,\"authMethod\":\"none\",\"apiProvider\":\"firstParty\",\"configDirectory\":%s}\n", q(cfg))
	return 1
}

// fakeSession records what an interactive session was started with, then
// behaves as FAKE_CLAUDE_* says.
func fakeSession(dir string, args []string, stdin io.Reader, on func(string) bool) int {
	argv := "\n"
	if len(args) > 0 {
		argv = strings.Join(args, "\n") + "\n"
	}
	fakeutil.WriteFile(filepath.Join(dir, "session.argv"), argv)
	fakeutil.WriteEnv(filepath.Join(dir, "session.env"))
	cwd, _ := os.Getwd()
	fakeutil.WriteFile(filepath.Join(dir, "session.cwd"), cwd+"\n")
	records, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "clalt", "sessions", "*.json"))
	var all []byte
	for _, p := range records {
		b, _ := fsutil.ReadFile(p)
		all = append(all, b...)
	}
	fakeutil.WriteFile(filepath.Join(dir, "session.records"), string(all))

	if on("SESSION_HOLDS") {
		hold(dir)
	}
	if on("SESSION_PRINTS") {
		fmt.Println("hello from claude")
		b, _ := io.ReadAll(stdin)
		fakeutil.WriteFile(filepath.Join(dir, "session.stdin"), string(b))
	}
	runExtras(dir)
	exit := os.Getenv("FAKE_CLAUDE_EXIT")
	if exit == "term" {
		fakeutil.KillSelf("TERM")
	}
	n, _ := strconv.Atoi(exit)
	return n
}

// hold waits for session.release, noting any TERM, HUP or INT on the way and
// ending with the code a shell would report for it. a signal that was ignored
// when the process started stays ignored, as it is for a shell.
func hold(dir string) {
	codes := map[os.Signal]int{syscall.SIGTERM: 143, syscall.SIGHUP: 129, os.Interrupt: 130}
	names := map[os.Signal]string{syscall.SIGTERM: "TERM", syscall.SIGHUP: "HUP", os.Interrupt: "INT"}
	ch := make(chan os.Signal, 4)
	for sig := range codes {
		if !signal.Ignored(sig) {
			signal.Notify(ch, sig)
		}
	}
	fakeutil.WriteFile(filepath.Join(dir, "session.pid"), strconv.Itoa(os.Getpid())+"\n")
	fakeutil.WriteFile(filepath.Join(dir, "session.holding"), "holding\n")
	tick := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case sig := <-ch:
			fakeutil.AppendLine(filepath.Join(dir, "signals"), names[sig])
			os.Exit(codes[sig])
		case <-tick.C:
			if _, err := os.Stat(filepath.Join(dir, "session.release")); err == nil {
				// default handling again, so a later KillSelf dies by signal
				signal.Reset()
				return
			}
		}
	}
}

func saveLogin(cfg, login, email, org, plan string) {
	_ = os.MkdirAll(cfg, 0o755)
	fakeutil.WriteFile(login, email+"\n"+org+"\n"+plan+"\n")
}

func main() {
	if fakeutil.Program() == "claude-guard" {
		os.Exit(fakeutil.Guard())
	}
	os.Exit(fakeClaudeMain(os.Args[1:], os.Stdin))
}
