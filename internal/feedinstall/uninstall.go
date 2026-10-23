package feedinstall

import (
	"fmt"
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// Result is what Uninstall did.
type Result struct {
	Removed  int  // handlers and statusLine taken out
	Restored bool // the file was put back byte for byte from the backup
	Deleted  bool // the file did not exist before install and is gone
	Backup   string
}

// Uninstall takes out what Apply added. a file that is still exactly what install
// wrote is put back from the backup, byte for byte (or removed, when install made
// it). one the user changed since is edited in place, see strip, after a backup
// of it.
func Uninstall(o Options) (Result, error) {
	raw, existed, err := inspect(o.Settings)
	if err != nil {
		return Result{}, err
	}
	st, haveState := loadState(o.StatePath)
	if !existed {
		_ = os.Remove(o.StatePath)
		return Result{}, nil
	}
	if haveState && st.Wrote == sum(raw) {
		res, err := restore(o, st)
		if err == nil {
			_ = os.Remove(o.StatePath)
		}
		return res, err
	}
	out, n, err := strip(raw, st.Original)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", o.Settings, err)
	}
	if n == 0 {
		_ = os.Remove(o.StatePath)
		return Result{}, nil
	}
	b, err := backup(o.BackupDir, o.Now, raw)
	if err != nil {
		return Result{}, err
	}
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(o.Settings); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := fsutil.WriteFileAtomic(o.Settings, out, mode); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", o.Settings, err)
	}
	_ = os.Remove(o.StatePath)
	return Result{Removed: n, Backup: b}, nil
}

func restore(o Options, st state) (Result, error) {
	if st.Created {
		if err := os.Remove(o.Settings); err != nil {
			return Result{}, err
		}
		return Result{Removed: 1, Deleted: true}, nil
	}
	orig, err := fsutil.ReadFile(st.Backup)
	if err != nil {
		return Result{}, fmt.Errorf("read the backup %s: %w", st.Backup, err)
	}
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(o.Settings); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := fsutil.WriteFileAtomic(o.Settings, orig, mode); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", o.Settings, err)
	}
	return Result{Removed: 1, Restored: true, Backup: st.Backup}, nil
}

// Status is what the settings file says about the feed.
type Status struct {
	Installed   bool
	Hooks       int  // events that have a handler of clalt's
	StatusLine  bool // the statusLine is clalt's
	Chained     bool // a statusLine of the user's runs through it
	Backup      string
	InstalledAt time.Time
	Bin         string // the clalt the commands run, from the state
	BinMissing  bool   // and it is not there any more
}

// Check reads the file, not the state: that is what claude runs.
func Check(o Options) (Status, error) {
	raw, existed, err := inspect(o.Settings)
	if err != nil || !existed {
		return Status{}, err
	}
	obj, _ := parseObject(raw)
	var s Status
	hooks, err := hooksOf(obj)
	if err != nil {
		return Status{}, err
	}
	for ev := range hooks {
		if groups, err := groupsOf(hooks, ev); err == nil && hasOurs(groups) {
			s.Hooks++
		}
	}
	s.StatusLine = obj["statusLine"] != nil && lineIsOurs(obj["statusLine"])
	s.Installed = s.Hooks > 0 || s.StatusLine
	if st, ok := loadState(o.StatePath); ok {
		s.Chained, s.Backup, s.InstalledAt, s.Bin = st.Original != nil, st.Backup, st.InstalledAt, st.Bin
		if _, err := os.Stat(st.Bin); st.Bin != "" && err != nil {
			s.BinMissing = true
		}
	}
	return s, nil
}
