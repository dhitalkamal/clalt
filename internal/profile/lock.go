package profile

import (
	"path/filepath"
	"time"

	"github.com/dhitalkamal/clalt/internal/fslock"
)

// syncLockName is the lock file inside an account folder that serializes the
// syncs of that account.
const syncLockName = ".clalt-sync.lock"

// syncLockTimeout is a var so a test can shorten it. a sync takes milliseconds,
// so waiting this long means a stuck process.
var syncLockTimeout = 10 * time.Second

// lockAccount serializes the syncs of one account folder across processes: two
// `clalt run` sessions of the same account start at the same moment. it creates
// the folder when needed. Plan and PlanMCP never take it, so the health check
// stays read-only.
func lockAccount(profileDir string) (unlock func(), err error) {
	l, err := fslock.Acquire(filepath.Join(profileDir, syncLockName), syncLockTimeout)
	if err != nil {
		return nil, err
	}
	return func() { _ = l.Release() }, nil
}
