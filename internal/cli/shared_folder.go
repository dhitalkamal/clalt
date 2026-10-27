package cli

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/config"
)

// foldersCollide says whether two account ids lead to the same folder. v0.1.0
// let ids differ only in case, and on a file system that ignores case, as macOS
// and Windows have by default, those are one folder and so one login. it looks
// at the real folders when it can, so that a file system that tells them apart
// is not refused for nothing. a variable so a test can force either answer.
var foldersCollide = realFoldersCollide

func realFoldersCollide(a, b string) bool {
	if a == b || !strings.EqualFold(a, b) {
		return false
	}
	// an id that is not a safe folder name is never turned into a path to look at
	dirA, errA := config.AccountDirFor(a)
	dirB, errB := config.AccountDirFor(b)
	if errA != nil || errB != nil {
		return false
	}
	fa, errA := os.Stat(dirA)
	fb, errB := os.Stat(dirB)
	switch {
	case errA == nil && errB == nil:
		return os.SameFile(fa, fb)
	case errA == nil || errB == nil:
		// one folder exists and the other does not: a file system that ignores
		// case would show both
		return false
	}
	// neither exists yet, so go by what the platform usually does
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}

// foldersSharedWith are the other visible accounts that have the same folder as
// acc. the store lists the ids that differ only in case. a removed account is
// not counted: it holds no login worth protecting, and counting it would leave
// no way out.
func (a *App) foldersSharedWith(acc account.Account) ([]account.Account, error) {
	groups, err := a.Store.CaseCollisions()
	if err != nil {
		return nil, err
	}
	var group []string
	for _, g := range groups {
		if slices.Contains(g, acc.Name) {
			group = g
			break
		}
	}
	visible, err := a.visibleAccounts()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]account.Account, len(visible))
	for _, v := range visible {
		byID[v.Name] = v
	}
	var out []account.Account
	for _, id := range group {
		if other, ok := byID[id]; ok && foldersCollide(acc.Name, id) {
			out = append(out, other)
		}
	}
	return out, nil
}

func sharedFolderMessage(acc account.Account, others []account.Account) string {
	names := make([]string, len(others))
	for i, o := range others {
		names[i] = fmt.Sprintf("%q", shown(o))
	}
	return fmt.Sprintf("%q and %s would share one folder: their ids differ only in case, and file names ignore case here. "+
		"remove one of them with 'clalt remove <name>', then sign the other in again", shown(acc), strings.Join(names, ", "))
}

// refuseSharedFolder is the error for an account that cannot be used while
// another one has its folder, or nil when it has the folder to itself.
func (a *App) refuseSharedFolder(acc account.Account) error {
	others, err := a.foldersSharedWith(acc)
	if err != nil || len(others) == 0 {
		return err
	}
	return fmt.Errorf("%s", sharedFolderMessage(acc, others))
}
