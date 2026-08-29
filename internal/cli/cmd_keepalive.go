package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/scheduler"
)

const (
	// keepaliveLabel is the launchd LaunchAgent label.
	keepaliveLabel = "com.clalt.keepalive"
	// defaults tuned so a token expiring before the next run is caught: the
	// buffer is wider than the interval on purpose.
	defKeepInterval = 30 * time.Minute
	defKeepBuffer   = 45 * time.Minute
)

// cmdKeepalive keeps every account's token refreshed ahead of expiry, whether or
// not the account is in use.
//
// usage:
//
//	clalt keepalive [--interval 30m] [--buffer 45m]   foreground loop (ctrl-c stops)
//	clalt keepalive --once [--buffer 45m]             one refresh pass, then exit
//	clalt keepalive install [--interval] [--buffer]   install background LaunchAgent
//	clalt keepalive uninstall                         remove the LaunchAgent
//	clalt keepalive status                            show agent + per-account expiry
func (a *App) cmdKeepalive(args []string) error {
	sub := ""
	if len(args) > 0 && !isFlag(args[0]) {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "install":
		return a.keepaliveInstall(args)
	case "uninstall", "remove":
		return a.keepaliveUninstall()
	case "status":
		return a.keepaliveStatus()
	case "":
		return a.keepaliveRun(args)
	default:
		return fmt.Errorf("unknown keepalive subcommand %q", sub)
	}
}

// keepaliveRun handles the foreground loop and the --once one-shot.
func (a *App) keepaliveRun(args []string) error {
	interval, buffer, once, _, err := parseKeepFlags(args)
	if err != nil {
		return err
	}
	if once {
		a.keepaliveReport(a.keepaliveTick(buffer))
		return nil
	}

	a.printf("keepalive started: every %s, refresh anything expiring within %s. ctrl-c to stop.\n", interval, buffer)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	a.keepaliveReport(a.keepaliveTick(buffer)) // run once immediately
	for {
		select {
		case <-stop:
			a.printf("keepalive stopped.\n")
			return nil
		case <-tick.C:
			a.keepaliveReport(a.keepaliveTick(buffer))
		}
	}
}

// keepaliveTick refreshes near-expiry accounts and stamps the result for logs.
type keepResult struct {
	refreshed []string
	errs      []error
}

func (a *App) keepaliveTick(buffer time.Duration) keepResult {
	r, errs := a.keepaliveOnce(buffer)
	return keepResult{refreshed: r, errs: errs}
}

// keepaliveOnce refreshes every account whose token expires within buffer (or is
// already expired). Accounts without a refresh token are skipped. Per-account
// errors are collected so one failure does not abort the rest.
func (a *App) keepaliveOnce(buffer time.Duration) (refreshed []string, errs []error) {
	accts, err := a.Store.List()
	if err != nil {
		return nil, []error{err}
	}
	for _, ac := range accts {
		cred, _, gerr := a.Store.Get(ac.Name)
		if gerr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ac.Name, gerr))
			continue
		}
		if cred.RefreshToken == "" {
			continue // nothing to refresh with
		}
		if !cred.IsExpired(buffer) {
			continue // still fresh enough
		}
		if _, rerr := a.refreshAccount(ac.Name); rerr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ac.Name, rerr))
			continue
		}
		refreshed = append(refreshed, ac.Name)
	}
	return refreshed, errs
}

func (a *App) keepaliveReport(r keepResult) {
	stamp := time.Now().Format("15:04:05")
	if len(r.refreshed) > 0 {
		a.printf("[%s] refreshed: %s\n", stamp, listOrNone(r.refreshed))
	}
	for _, e := range r.errs {
		a.printf("[%s] error: %v\n", stamp, e)
	}
	if len(r.refreshed) == 0 && len(r.errs) == 0 {
		a.printf("[%s] all tokens still fresh\n", stamp)
	}
}

// keepaliveJob builds the scheduler Job that runs `clalt keepalive --once`
// periodically, resolving the clalt binary path and the log directory.
func (a *App) keepaliveJob(interval, buffer time.Duration) (scheduler.Job, error) {
	bin, err := os.Executable()
	if err != nil {
		return scheduler.Job{}, fmt.Errorf("resolve clalt binary path: %w", err)
	}
	logDir := filepath.Join(config.ClaltDir(), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return scheduler.Job{}, fmt.Errorf("create log dir: %w", err)
	}
	return scheduler.Job{
		Label:      keepaliveLabel,
		Program:    bin,
		Args:       []string{"keepalive", "--once", "--buffer", buffer.String()},
		Interval:   interval,
		StdoutPath: filepath.Join(logDir, "keepalive.log"),
		StderrPath: filepath.Join(logDir, "keepalive.err"),
		RunAtLoad:  true, // refresh immediately, not just after the first interval
	}, nil
}

// keepaliveInstall registers the background job with the host scheduler
// (launchd on macOS, systemd user timer on Linux, Task Scheduler on Windows).
// With --dry-run it prints exactly what would be written and runs nothing.
func (a *App) keepaliveInstall(args []string) error {
	interval, buffer, _, dryRun, err := parseKeepFlags(args)
	if err != nil {
		return err
	}
	sched, err := scheduler.For(runtime.GOOS)
	if err != nil {
		return err
	}
	job, err := a.keepaliveJob(interval, buffer)
	if err != nil {
		return err
	}
	if dryRun {
		a.printf("%s\n", sched.Plan(job))
		return nil
	}
	if err := sched.Install(job); err != nil {
		return err
	}
	// a manual install counts as handled and clears any prior opt-out
	_ = touchFile(config.KeepaliveAutoMarkerPath())
	_ = os.Remove(config.KeepaliveDisabledPath())
	a.printf("installed background keepalive (%s): every %s, buffer %s\n", sched.Name(), interval, buffer)
	a.printf("  binary: %s\n", job.Program)
	a.printf("  logs:   %s\n", job.StdoutPath)
	a.printf("  job:    %s\n", sched.Location(keepaliveLabel))
	return nil
}

func (a *App) keepaliveUninstall() error {
	sched, err := scheduler.For(runtime.GOOS)
	if err != nil {
		return err
	}
	if err := sched.Uninstall(keepaliveLabel); err != nil {
		return err
	}
	// remember the opt-out so auto-keepalive does not silently re-install it
	_ = os.Remove(config.KeepaliveAutoMarkerPath())
	_ = touchFile(config.KeepaliveDisabledPath())
	a.printf("removed background keepalive and disabled auto-enable (%s)\n", keepaliveLabel)
	a.printf("re-enable any time with 'clalt keepalive install'\n")
	return nil
}

func (a *App) keepaliveStatus() error {
	sched, err := scheduler.For(runtime.GOOS)
	if err != nil {
		return err
	}
	loaded, err := sched.Status(keepaliveLabel)
	if err != nil {
		return err
	}
	a.printf("background agent: %s\n", loadedText(loaded))
	a.printf("  backend: %s\n", sched.Name())
	a.printf("  job:     %s\n", sched.Location(keepaliveLabel))
	accts, err := a.Store.List()
	if err != nil {
		return err
	}
	a.printf("token expiry:\n")
	for _, ac := range accts {
		cred, _, gerr := a.Store.Get(ac.Name)
		if gerr != nil {
			a.printf("  %-16s (no secret)\n", ac.Name)
			continue
		}
		a.printf("  %-16s %s\n", ac.Name, humanUntil(cred.ExpiresAtTime()))
	}
	return nil
}

// parseKeepFlags parses --interval, --buffer, --once, and --dry-run from a
// keepalive invocation.
func parseKeepFlags(args []string) (interval, buffer time.Duration, once, dryRun bool, err error) {
	interval, buffer = defKeepInterval, defKeepBuffer
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--once":
			once = true
		case "--dry-run":
			dryRun = true
		case "--interval":
			if i+1 >= len(args) {
				return 0, 0, false, false, errors.New("--interval needs a duration, e.g. 30m")
			}
			i++
			if interval, err = time.ParseDuration(args[i]); err != nil {
				return 0, 0, false, false, fmt.Errorf("bad interval %q", args[i])
			}
		case "--buffer":
			if i+1 >= len(args) {
				return 0, 0, false, false, errors.New("--buffer needs a duration, e.g. 45m")
			}
			i++
			if buffer, err = time.ParseDuration(args[i]); err != nil {
				return 0, 0, false, false, fmt.Errorf("bad buffer %q", args[i])
			}
		default:
			return 0, 0, false, false, fmt.Errorf("unknown keepalive flag %q", args[i])
		}
	}
	return interval, buffer, once, dryRun, nil
}

func isFlag(s string) bool { return len(s) > 0 && s[0] == '-' }

func loadedText(loaded bool) string {
	if loaded {
		return "loaded (running in background)"
	}
	return "not installed"
}
