package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
)

// keepaliveLabel is what the retired keepalive job was installed under.
const keepaliveLabel = "com.clalt.keepalive"

// leftoverChecks look for what the old design left on disk. they only report:
// deleting somebody's files is for them to decide.
func (a *App) leftoverChecks() []checkResult {
	var checks []checkResult
	// a check looks in places under the home. with no home they are relative
	// paths, which is the working directory and not what the check is about
	add := func(known bool, name string, check func() checkResult) {
		if known {
			checks = append(checks, check())
			return
		}
		checks = append(checks, result("leftover", name, statusWarn, "not checked: %s", noHomeKnown))
	}
	claltKnown := filepath.IsAbs(config.ClaltDir())
	add(claltKnown, "oldVault", vaultCheck)
	add(claltKnown, "oldProfiles", a.profilesCheck)
	if goos := runtime.GOOS; goos == "darwin" || goos == "linux" {
		base := homeDir()
		if goos == "linux" {
			base = configHome()
		}
		add(filepath.IsAbs(base), "oldKeepalive", func() checkResult { return keepaliveCheck(goos) })
	}
	add(claltKnown, "oldKeepaliveMarkers", func() checkResult { return markersCheck(runtime.GOOS) })
	if oldCredentialsMatter(runtime.GOOS) {
		checks = append(checks, credentialsCheck())
	}
	return checks
}

func vaultCheck() checkResult {
	var found []string
	for _, p := range []string{config.VaultDir(), config.VaultKeyPath()} {
		if exists(p) {
			found = append(found, p)
		}
	}
	if len(found) == 0 {
		return result("leftover", "oldVault", statusOK, "no old token vault")
	}
	return result("leftover", "oldVault", statusWarn,
		"the old token vault is still on disk: %s. nothing reads it now, delete it when you no longer need it", strings.Join(found, ", "))
}

// oldKeepaliveFiles lists the files of the retired keepalive job that are still
// there. windows kept it as a scheduled task, not as a file, so there is
// nothing to look for.
func oldKeepaliveFiles(goos string) []string {
	var candidates []string
	switch goos {
	case "darwin":
		candidates = []string{filepath.Join(homeDir(), "Library", "LaunchAgents", keepaliveLabel+".plist")}
	case "linux":
		units := filepath.Join(configHome(), "systemd", "user")
		candidates = []string{filepath.Join(units, keepaliveLabel+".service"), filepath.Join(units, keepaliveLabel+".timer")}
	}
	var found []string
	for _, p := range candidates {
		if exists(p) {
			found = append(found, p)
		}
	}
	return found
}

func keepaliveCheck(goos string) checkResult {
	found := oldKeepaliveFiles(goos)
	if len(found) == 0 {
		return result("leftover", "oldKeepalive", statusOK, "no old keepalive job")
	}
	how := "'launchctl unload' it, then delete the file"
	if goos == "linux" {
		how = "run 'systemctl --user disable --now " + keepaliveLabel + ".timer', then delete the files"
	}
	return result("leftover", "oldKeepalive", statusWarn,
		"the old keepalive job is still installed: %s. it does nothing useful now. %s", strings.Join(found, ", "), how)
}

// oldCredentialsMatter is true where a .credentials.json in the claude home may
// be a leftover of the old switch flow. on linux claude keeps its real login in
// that file, so it is never a leftover there.
func oldCredentialsMatter(goos string) bool { return goos == "darwin" }

// credentialsCheck only looks whether the file is there. its content is a
// login and is never read.
func credentialsCheck() checkResult {
	shared := config.SharedClaudeDir()
	if shared == "" {
		return result("leftover", "oldCredentials", statusWarn, "not checked: %s", noHomeKnown)
	}
	path := filepath.Join(shared, ".credentials.json")
	if !exists(path) {
		return result("leftover", "oldCredentials", statusOK, "no old credentials file")
	}
	// on a mac claude keeps its login in the keychain, but it writes this file
	// itself when the keychain write fails, so it is not always a leftover
	return result("leftover", "oldCredentials", statusWarn,
		"%s is there. the old switch flow may have written it, but claude writes it too when it cannot save its login in the keychain, "+
			"so it can be your real login. do not delete it blindly: run 'claude auth status' from a normal shell. "+
			"if that says you are not signed in, the file is a leftover and can go. if it says you are, keep the file unless you know it is not your login", path)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

// configHome is where systemd looks for user units: XDG_CONFIG_HOME, else
// ~/.config.
func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".config")
}

// markersCheck lists the two files v0.1.0 kept in clalt's folder for the
// keepalive job: keepalive.auto when it set the job up by itself, and
// keepalive.disabled when the user turned it off. on windows the job was a
// scheduled task, which no file shows, so the first marker is the only trace
// that there may be one.
func markersCheck(goos string) checkResult {
	var found []string
	for _, name := range []string{"keepalive.auto", "keepalive.disabled"} {
		if path := filepath.Join(config.ClaltDir(), name); exists(path) {
			found = append(found, path)
		}
	}
	if len(found) == 0 {
		return result("leftover", "oldKeepaliveMarkers", statusOK, "no old keepalive marker files")
	}
	detail := "old keepalive marker files are still there: " + strings.Join(found, ", ") + ". nothing reads them, delete them when you like"
	if goos == "windows" && exists(filepath.Join(config.ClaltDir(), "keepalive.auto")) {
		detail += ". the job itself was a Task Scheduler task that no file shows. remove it with: schtasks /Delete /TN " + keepaliveLabel + " /F"
	}
	return result("leftover", "oldKeepaliveMarkers", statusWarn, "%s", detail)
}
