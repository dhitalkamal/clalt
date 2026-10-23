package crew

import (
	"errors"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/config"
)

// Root is the folder that holds everything clalt keeps about its gateways. the
// account folders belong to claude and hold nothing of this.
func Root() string { return filepath.Join(config.ClaltDir(), "crew") }

// LockPath is the lock that start and stop hold. ports are given out across all
// gateways, so one lock covers them all.
func LockPath() string { return filepath.Join(Root(), ".lock") }

// Paths are the places that belong to the gateway of one account.
type Paths struct {
	Dir       string // the account's folder under Root
	Record    string // gateway.json, there while a gateway is recorded
	Log       string // gateway.out, what the current run printed
	PrevLog   string // gateway.out.prev, the run before it
	Home      string // KIROCREW_HOME
	KiroHome  string // KIRO_HOME, inside Home, the layout Kiro Crew accepts as private
	Workspace string // KIROCREW_WORKSPACE
	// LocalConfig is config.local.json of the home, where the defaults are pinned
	LocalConfig string
}

// PathsFor works out the places of an account id. the id becomes a folder name,
// so it passes the same check every such id passes, and with no home nothing is
// made relative to wherever clalt happens to run.
func PathsFor(id string) (Paths, error) {
	if err := config.CheckAccountID(id); err != nil {
		return Paths{}, err
	}
	root := Root()
	if !filepath.IsAbs(root) {
		return Paths{}, errors.New("clalt's folder is not an absolute path (is HOME set?)")
	}
	dir := filepath.Join(root, id)
	home := filepath.Join(dir, "home")
	return Paths{
		Dir:         dir,
		Record:      filepath.Join(dir, recordFile),
		Log:         filepath.Join(dir, logFile),
		PrevLog:     filepath.Join(dir, prevLogFile),
		LocalConfig: filepath.Join(home, localConfig),
		Home:        home,
		KiroHome:    filepath.Join(home, "kiro"),
		Workspace:   filepath.Join(home, "workspace"),
	}, nil
}
