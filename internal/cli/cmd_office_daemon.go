package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/officed"
	"github.com/dhitalkamal/clalt/internal/reviews"
	"github.com/dhitalkamal/clalt/internal/version"
)

const officeDaemonUsage = "clalt office on|off|open|status [--link]|autostart on|off|status"

// mgr is the background office: the injected one in tests, else this machine.
func (a *App) mgr() officed.Manager {
	var m officed.Manager
	if a.officeMgr != nil {
		m = *a.officeMgr
	} else {
		exe, _ := os.Executable()
		m = officed.Manager{
			Dir: config.OfficeDir(), GOOS: runtime.GOOS, Version: version.Version,
			Now: a.clock, Verify: officed.Probe, Alive: officed.Alive, Stop: officed.Terminate,
			Spawn: func(log string) (officed.Child, error) { return officed.Spawn(exe, []string{"office", "serve"}, log) },
			Open:  a.officeOpener(), Sleep: time.Sleep,
			Missing: func() []officed.MissingTool {
				claude := os.Getenv("CLALT_CLAUDE_BIN")
				if claude == "" {
					claude = "claude"
				}
				return missingTools(reviews.DefaultResolver(), claude)
			},
		}
	}
	m.Out = a.Out
	return m
}

func (a *App) officeOpener() func(string) error {
	if a.officeOpen != nil {
		return a.officeOpen
	}
	return office.OpenBrowser
}

// autostart is start at login: the injected one in tests, else this machine.
func (a *App) autostart() officed.Autostart {
	if a.officeAuto != nil {
		return *a.officeAuto
	}
	exe, _ := os.Executable()
	return officed.Autostart{
		Home: config.Home(), UID: os.Getuid(), Exe: exe, LogPath: officed.LogPath(config.OfficeDir()), GOOS: runtime.GOOS,
		PATH: os.Getenv("PATH"),
		Run:  func(args []string) ([]byte, error) { return exec.Command("launchctl", args...).CombinedOutput() },
	}
}

// pageAutostart is the settings dialog's view of start at login.
type pageAutostart struct{ a officed.Autostart }

func (p pageAutostart) Status() office.AutostartStatus {
	return office.AutostartStatus{Supported: p.a.Supported(), Enabled: p.a.Enabled()}
}

// Set turns it on or off. a server that launchd itself runs must not boot its
// own job out, that would end the page that asked: it only removes the file.
func (p pageAutostart) Set(on bool) error {
	if on {
		return p.a.Enable()
	}
	return p.a.Disable(os.Getenv("XPC_SERVICE_NAME") != officed.Label)
}

// cmdOfficeDaemon runs the on, off, open, status, autostart and serve
// subcommands. handled is false for anything else.
func (a *App) cmdOfficeDaemon(sub string, args []string) (handled bool, err error) {
	switch sub {
	case "on", "off", "open":
		if len(args) > 0 {
			return true, usageError{"clalt office " + sub + " takes no arguments", officeDaemonUsage}
		}
		m := a.mgr()
		return true, map[string]func() error{"on": m.On, "off": m.Off, "open": m.OpenLink}[sub]()
	case "status":
		if len(args) > 1 || (len(args) == 1 && args[0] != "--link") {
			return true, usageError{"clalt office status takes only --link", officeDaemonUsage}
		}
		return true, a.mgr().Status(len(args) == 1)
	case "autostart":
		return true, a.cmdOfficeAutostart(args)
	case "serve":
		return true, a.cmdOfficeServe(args)
	}
	return false, nil
}

func (a *App) cmdOfficeAutostart(args []string) error {
	if len(args) != 1 || (args[0] != "on" && args[0] != "off" && args[0] != "status") {
		return usageError{"clalt office autostart needs on, off or status", officeDaemonUsage}
	}
	au := a.autostart()
	switch args[0] {
	case "on":
		if err := au.Enable(); err != nil {
			return err
		}
		a.printf("the office will start when you log in (%s)\n", au.PlistPath())
	case "off":
		if err := au.Disable(true); err != nil {
			return err
		}
		a.printf("the office will no longer start when you log in\n")
	default:
		switch {
		case !au.Supported():
			a.printf("start at login: not available (macOS only for now)\n")
		case au.Enabled():
			a.printf("start at login: on\n")
		default:
			a.printf("start at login: off\n")
		}
	}
	return nil
}

// portFree says whether 127.0.0.1:port can be listened on right now.
func portFree(port int) bool {
	l, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// cmdOfficeServe is the process behind clalt office on and start at login:
// the office server without a browser, which writes the run file once it
// listens. its output goes to office.log, so the token is never printed.
func (a *App) cmdOfficeServe(args []string) error {
	opt, err := parseOfficeArgs(args)
	if err != nil {
		return err
	}
	m := a.mgr()
	if r, ok := m.Running(); ok {
		officeLogf(dashboardLogger(officeLogPath()))("serve: already running on port %d (pid %d), not starting a second one", r.Port, r.PID)
		return nil
	}
	if opt.port == 0 {
		if opt.port, err = officed.PickPort(officed.DefaultPort, officed.PortRange, portFree); err != nil {
			return err
		}
	}
	opt.noOpen = true
	pid := os.Getpid()
	defer officed.RemoveRun(m.Dir, pid)
	return a.runOffice(opt, io.Discard, func(port int, token string) {
		run := officed.Run{PID: pid, Port: port, Token: token, StartedAt: a.clock(), Version: version.Version}
		if err := officed.WriteRun(m.Dir, run); err != nil {
			officeLogf(dashboardLogger(officeLogPath()))("serve: could not write the run file: %v", err)
		}
	})
}
