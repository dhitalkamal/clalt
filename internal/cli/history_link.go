package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// historyState says how an account's projects entry stands against the shared
// projects folder.
type historyState int

const (
	historyShared      historyState = iota // a link that leads to the shared projects folder
	historyOwn                             // a real folder or file: the history would be the account's own
	historyUnreachable                     // nothing there, or a link that does not lead to the shared folder
)

// historyOf looks at <folder>/projects itself, whatever the planner did or did
// not do. claude makes a real projects folder when there is none, so anything
// but a link that reaches the shared folder would fork the history. a link
// into a volume that is not mounted does not reach it: nothing is there to open.
func historyOf(folder string) historyState {
	link := filepath.Join(folder, "projects")
	fi, err := os.Lstat(link)
	switch {
	case err != nil:
		return historyUnreachable
	case fi.Mode()&os.ModeSymlink == 0:
		return historyOwn
	}
	shared := config.SharedClaudeDir()
	if shared == "" || !fsutil.SamePath(link, filepath.Join(shared, "projects")) {
		return historyUnreachable
	}
	return historyShared
}

// warnHistory is for login, right after the sync: when the account's history
// does not reach the shared one, run will refuse it, and login says so once
// instead of leaving the first run to find out. it is not a refusal, the
// sign-in goes on and doctor has the details.
func (a *App) warnHistory(label, folder string) {
	if historyOf(folder) == historyShared {
		return
	}
	fmt.Fprintf(a.Err, "warning: the history of %s does not reach the shared history folder %s, so 'clalt run %s' will refuse until it does. 'clalt doctor' says why\n",
		label, clean(filepath.Join(config.SharedClaudeDir(), "projects")), label)
}
