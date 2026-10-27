package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/defaulthome"
	"github.com/dhitalkamal/clalt/internal/feedinstall"
	"github.com/dhitalkamal/clalt/internal/usage"
)

const feedUsage = "clalt feed install|uninstall [--yes] | status [--json] | forget-default | remember-default"

// feedInstallPath is where clalt keeps what uninstall needs besides the backup.
func feedInstallPath() string { return filepath.Join(config.ClaltDir(), "feed-install.json") }

// cmdFeed is `clalt feed`: the opt-in live feed for plain claude, which adds
// clalt's hooks and statusline to the shared settings.json, and the control of
// how plain claude's usage is attributed.
func (a *App) cmdFeed(args []string) error {
	if len(args) == 0 {
		return usageError{"feed needs install, uninstall, status, forget-default or remember-default", feedUsage}
	}
	sub, rest := args[0], args[1:]
	yes := false
	rest = slices.DeleteFunc(rest, func(s string) bool {
		if s == "--yes" || s == "-y" {
			yes = true
			return true
		}
		return false
	})
	if sub == "status" {
		return a.feedStatus(rest)
	}
	if len(rest) > 0 {
		return usageError{"feed " + sub + " takes no arguments but --yes", feedUsage}
	}
	switch sub {
	case "install":
		return a.feedInstall(yes)
	case "uninstall":
		return a.feedUninstall(yes)
	case "forget-default":
		return a.feedAttribution(false)
	case "remember-default":
		return a.feedAttribution(true)
	}
	return usageError{fmt.Sprintf("unknown feed command %q", sub), feedUsage}
}

// feedOptions is the install of this machine: the settings.json of the shared
// claude home and the running clalt.
func (a *App) feedOptions() (feedinstall.Options, error) {
	home := config.SharedClaudeDir()
	if home == "" {
		return feedinstall.Options{}, errors.New(noHomeKnown)
	}
	if err := feedSupported(runtime.GOOS); err != nil {
		return feedinstall.Options{}, err
	}
	bin, err := os.Executable()
	if err == nil {
		bin, err = filepath.Abs(bin)
	}
	if err != nil {
		return feedinstall.Options{}, fmt.Errorf("clalt could not find itself: %w", err)
	}
	bin, note := stableBinary(bin, exec.LookPath)
	if note != "" {
		a.printNote(note)
	}
	evs := make([]feedinstall.Event, len(hookEvents))
	for i, e := range hookEvents {
		evs[i] = feedinstall.Event{Name: e.name, Matcher: e.matcher, Timeout: e.timeout, Async: e.async}
	}
	return feedinstall.Options{
		Settings: filepath.Join(home, "settings.json"), BackupDir: filepath.Join(config.ClaltDir(), "backups", "settings"),
		StatePath: feedInstallPath(), Protected: []string{config.ClaltDir()}, Bin: bin, Quote: shQuote, Events: evs, Now: a.clock(),
	}, nil
}

func (a *App) feedInstall(yes bool) error {
	opts, err := a.feedOptions()
	if err != nil {
		return err
	}
	c, err := a.refreshDefaultHome()
	if err != nil {
		return err
	}
	if !c.Active() {
		return fmt.Errorf("plain claude is not tied to an account, so the feed would have nothing to record it under: %s", clean(c.Reason))
	}
	plan, err := feedinstall.Prepare(opts)
	if err != nil {
		return cleanErr(err)
	}
	if !plan.Changed {
		a.printf("the feed is already installed in %s, nothing to change\n", clean(opts.Settings))
		return nil
	}
	a.linkNotice(plan.Target(), opts.Settings)
	a.printf("this adds clalt's hooks (%d) and statusline to %s, so plain claude reports to clalt as %s.\n", plan.Hooks, clean(opts.Settings), clean(a.accountLabel(c.Account)))
	a.printf("your own hooks stay, and your statusLine keeps running through clalt. a copy of the file is kept first.\n")
	if !yes {
		ok, err := a.confirm("install the feed?")
		if err != nil {
			return fmt.Errorf("not installed: nobody answered, pass --yes to install without asking")
		}
		if !ok {
			a.printf("not installed\n")
			return nil
		}
	}
	if err := plan.Apply(); err != nil {
		return cleanErr(err)
	}
	a.printf("installed in %s\n", clean(opts.Settings))
	if plan.BackupPath != "" {
		a.printf("backup of the old file: %s\n", clean(plan.BackupPath))
	}
	a.printf("remove it with: clalt feed uninstall\n")
	return nil
}

func (a *App) feedUninstall(yes bool) error {
	opts, err := a.feedOptions()
	if err != nil {
		return err
	}
	st, err := feedinstall.Check(opts)
	if err != nil {
		return cleanErr(err)
	}
	if st.Link {
		a.printf("settings.json is a link to %s\n", clean(st.LinkTarget))
		if st.LinkProblem != "" {
			a.printf("clalt cannot use it: %s\n", clean(st.LinkProblem))
		}
	}
	if !st.Installed {
		a.printf("the feed is not installed in %s\n", clean(opts.Settings))
		return nil
	}
	t, err := feedinstall.ResolveTarget(opts)
	if err != nil {
		return cleanErr(err)
	}
	a.linkNotice(t, opts.Settings)
	if !yes {
		ok, err := a.confirm("remove clalt's hooks and statusline from " + clean(opts.Settings) + "?")
		if err != nil {
			return fmt.Errorf("not removed: nobody answered, pass --yes to remove without asking")
		}
		if !ok {
			a.printf("not removed\n")
			return nil
		}
	}
	res, err := feedinstall.Uninstall(opts)
	if err != nil {
		return cleanErr(err)
	}
	switch {
	case res.Deleted:
		a.printf("removed %s, which only held clalt's entries\n", clean(opts.Settings))
	case res.Restored:
		a.printf("restored %s from the backup, byte for byte\n", clean(opts.Settings))
	default:
		a.printf("removed clalt's entries from %s (keys sorted, two space indent)\n", clean(opts.Settings))
		a.printf("backup of the file before this: %s\n", clean(res.Backup))
	}
	return nil
}

type feedStatusJSON struct {
	Installed  bool   `json:"installed"`
	Settings   string `json:"settings"`
	Hooks      int    `json:"hooks"`
	StatusLine bool   `json:"statusLine"`
	Chained    bool   `json:"chainsYourStatusLine"`
	Backup     string `json:"backup,omitempty"`
	Account    string `json:"account,omitempty"`
	Default    string `json:"defaultHome"`
	Reason     string `json:"reason,omitempty"`
	LinkTarget string `json:"settingsLinkTarget,omitempty"`
	LinkIssue  string `json:"settingsLinkProblem,omitempty"`
}

func (a *App) feedStatus(rest []string) error {
	if len(rest) > 0 {
		return usageError{"feed status takes no arguments but --json", feedUsage}
	}
	opts, err := a.feedOptions()
	if err != nil {
		return err
	}
	st, err := feedinstall.Check(opts)
	if err != nil {
		return cleanErr(err)
	}
	c, err := a.refreshDefaultHome()
	if err != nil {
		return err
	}
	out := feedStatusJSON{
		Installed: st.Installed, Settings: opts.Settings, Hooks: st.Hooks, StatusLine: st.StatusLine, Chained: st.Chained,
		Backup: st.Backup, Account: c.Account, Default: string(c.State), Reason: clean(c.Reason),
		LinkTarget: clean(st.LinkTarget), LinkIssue: clean(st.LinkProblem),
	}
	if a.JSON {
		return a.emitJSON(out)
	}
	if st.Link {
		a.printf("settings.json is a link to %s\n", clean(st.LinkTarget))
		if st.LinkProblem != "" {
			a.printf("clalt cannot use it: %s\n", clean(st.LinkProblem))
		}
	}
	if !st.Installed {
		a.printf("the feed is not installed in %s\n", clean(opts.Settings))
	} else {
		a.printf("the feed is installed in %s: %d hooks, statusline %s\n", clean(opts.Settings), st.Hooks, yesNo(st.StatusLine))
		if st.Chained {
			a.printf("your own statusLine runs through clalt's\n")
		}
		if st.Backup != "" {
			a.printf("backup from before the install: %s\n", clean(st.Backup))
		}
		if st.BinMissing {
			a.printf("warning: the clalt these commands run, %s, no longer exists, so the feed does nothing: run clalt feed uninstall, then clalt feed install\n", clean(st.Bin))
		}
	}
	a.printf("%s\n", defaultHomeText(c, a.accountLabel(c.Account)))
	return nil
}

func yesNo(b bool) string {
	if b {
		return "set"
	}
	return "not set"
}

// defaultHomeText says in words how plain claude relates to the accounts.
func defaultHomeText(c defaulthome.Cache, label string) string {
	switch {
	case c.Active():
		s := "plain claude (~/.claude) is signed in as " + clean(label)
		if c.Off {
			s += ", usage attribution is off"
		}
		return s
	case c.State == defaulthome.Unusable:
		return "plain claude (~/.claude) could not be checked: " + clean(c.Reason)
	}
	return "plain claude (~/.claude) stays outside clalt: " + clean(c.Reason)
}

// feedAttribution turns the attribution of plain claude's usage off, taking back
// the ledger lines clalt wrote for it, or on again.
func (a *App) feedAttribution(on bool) error {
	c, _ := defaulthome.Read(config.DefaultHomePath())
	if c.State == "" {
		c = defaulthome.Cache{State: defaulthome.Outside, Reason: "not checked yet", CheckedAt: a.clock()}
	}
	c.Off = !on
	if err := os.MkdirAll(config.ClaltDir(), 0o700); err != nil {
		return err
	}
	if err := defaulthome.Write(config.DefaultHomePath(), c); err != nil {
		return err
	}
	if on {
		a.printf("usage of plain claude is attributed to its account again, from the next report\n")
		return nil
	}
	n, err := usage.RemoveSource(config.SessionAccountsPath(), usage.SourceDefault)
	if err != nil {
		return err
	}
	a.printf("removed %d session lines that clalt attributed to the default home; that usage is unattributed again\n", n)
	return nil
}
