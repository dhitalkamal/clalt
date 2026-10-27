package feedinstall

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// Options say what to edit and where clalt keeps its side of it.
type Options struct {
	Settings  string // settings.json of the shared claude home
	BackupDir string // ~/.config/clalt/backups/settings
	StatePath string // ~/.config/clalt/feed-install.json
	Bin       string // the running clalt, absolute
	// Quote quotes Bin for the shell that claude runs hooks with, single quotes
	// when nil
	Quote  func(string) string
	Events []Event
	Now    time.Time
	// Protected are clalt's own folders: a link into one is refused
	Protected []string
}

func (o Options) quote(s string) string {
	if o.Quote != nil {
		return o.Quote(s)
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// state is feed-install.json: what uninstall needs that settings.json cannot
// hold.
type state struct {
	InstalledAt time.Time       `json:"installedAt"`
	Settings    string          `json:"settings"`
	Target      string          `json:"target,omitempty"` // the file install edited, after links
	Backup      string          `json:"backup,omitempty"`
	Bin         string          `json:"bin,omitempty"`     // the clalt the commands run
	Created     bool            `json:"created,omitempty"` // there was no settings.json
	Wrote       string          `json:"wrote"`             // sha256 of what install wrote
	Original    json.RawMessage `json:"originalStatusLine,omitempty"`
}

// Plan is an install that is worked out and not yet written.
type Plan struct {
	opts       Options
	raw        []byte
	existed    bool
	m          merged
	Changed    bool // there is something to add
	Hooks      int  // handlers it adds
	StatusLine bool // it sets the statusLine
	Chained    bool // the user's statusLine will run through clalt's
	BackupPath string
	target     Target
}

// Target is the file this plan edits.
func (p Plan) Target() Target { return p.target }

// inspect reads the settings file at path, which is already resolved past any
// link: it refuses what is not a regular file and what is not a JSON object.
func inspect(path string) (raw []byte, existed bool, err error) {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !fi.Mode().IsRegular() {
		return nil, true, fmt.Errorf("%s is not a regular file", path)
	}
	raw, err = fsutil.ReadFile(path)
	if err != nil {
		return nil, true, err
	}
	if _, perr := parseObject(raw); perr != nil {
		return nil, true, fmt.Errorf("%s is %v: fix it first, clalt changed nothing", path, perr)
	}
	return raw, true, nil
}

// Prepare works out the edit and changes nothing.
func Prepare(o Options) (Plan, error) {
	t, err := ResolveTarget(o)
	if err != nil {
		return Plan{}, err
	}
	raw, existed, err := inspect(t.Path)
	if err != nil {
		return Plan{}, err
	}
	m, err := merge(raw, o.Bin, o.quote, o.Events)
	if err != nil {
		return Plan{}, fmt.Errorf("%s: %w", o.Settings, err)
	}
	if err := checkParses(m.Out); err != nil {
		return Plan{}, err
	}
	p := Plan{opts: o, target: t, raw: raw, existed: existed, m: m, Changed: m.changed(), Hooks: m.Hooks, StatusLine: m.Line}
	p.Chained = m.Original != nil
	return p, nil
}

// Apply backs the old file up, then writes the new one. nothing is written when
// the backup cannot be, and a plan with nothing to add writes nothing at all.
func (p *Plan) Apply() error {
	if !p.Changed {
		return nil
	}
	o := p.opts
	// the question may have been on screen for a while, and claude saves this
	// file too: what was worked out is only good for the bytes it was worked from
	t, err := ResolveTarget(o)
	if err != nil {
		return err
	}
	if t.Path != p.target.Path {
		return errors.New("settings.json points somewhere else now, run clalt feed install again")
	}
	now, existedNow, err := inspect(t.Path)
	if err != nil {
		return err
	}
	if existedNow != p.existed || !bytes.Equal(now, p.raw) {
		return errors.New("settings.json changed while waiting, run clalt feed install again")
	}
	st := state{Bin: o.Bin, InstalledAt: o.Now.UTC(), Settings: o.Settings, Target: t.Path, Created: !p.existed, Wrote: sum(p.m.Out), Original: p.m.Original}
	prev, hadPrev := loadState(o.StatePath)
	if hadPrev && st.Original == nil {
		// a second pass that adds a missing event must not forget the first
		st.Original, st.Created = prev.Original, prev.Created
	}
	if hadPrev && p.existed && prev.Wrote == sum(p.raw) {
		// the file is still what the first install wrote, so the first backup is
		// still the user's
		st.Backup = prev.Backup
	} else if p.existed {
		b, err := backup(o.BackupDir, o.Now, p.raw)
		if err != nil {
			return err
		}
		p.BackupPath, st.Backup = b, b
	}
	if err := saveState(o.StatePath, st); err != nil {
		return err
	}
	undoState := func() {
		if hadPrev {
			_ = saveState(o.StatePath, prev)
		} else {
			_ = os.Remove(o.StatePath)
		}
	}
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(t.Path); err == nil {
		mode = fi.Mode().Perm()
	}
	// write beside the real file, never over the link
	if err := fsutil.WriteFileAtomic(t.Path, p.m.Out, mode); err != nil {
		undoState()
		return fmt.Errorf("write %s: %w", t.Path, err)
	}
	return nil
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func backup(dir string, now time.Time, raw []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("make the backup folder: %w", err)
	}
	path := filepath.Join(dir, "settings-"+now.UTC().Format("20060102T150405Z")+".json")
	// two backups in one second would overwrite each other
	for i := 1; ; i++ {
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("settings-%s-%d.json", now.UTC().Format("20060102T150405Z"), i))
	}
	if err := fsutil.WriteFileAtomic(path, raw, 0o600); err != nil {
		return "", fmt.Errorf("write the backup: %w", err)
	}
	return path, nil
}

func loadState(path string) (state, bool) {
	raw, err := fsutil.ReadFile(path)
	var st state
	if err != nil || json.Unmarshal(raw, &st) != nil {
		return state{}, false
	}
	return st, true
}

func saveState(path string, st state) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(raw, '\n'), 0o600)
}

// LoadOriginal is the statusLine of the user's that install replaced, from
// clalt's own state. clalt statusline runs its command.
func LoadOriginal(statePath string) (json.RawMessage, bool) {
	st, ok := loadState(statePath)
	return st.Original, ok && st.Original != nil
}

// checkParses runs the statusLine command that is about to be written through
// sh -n, which only parses. a command that does not parse would blank the line
// while install said it worked. without an sh to ask, nothing is checked.
func checkParses(out []byte) error {
	sh, err := exec.LookPath("sh")
	if err != nil || out == nil {
		return nil
	}
	o, err := parseObject(out)
	if err != nil {
		return nil
	}
	var line handler
	if json.Unmarshal(o["statusLine"], &line) != nil || !hasMarker(line.Command) {
		return nil
	}
	if err := exec.Command(sh, "-n", "-c", line.Command).Run(); err != nil {
		return errors.New("the statusLine command clalt would write does not parse as shell (your own statusLine command is probably the cause): nothing was changed")
	}
	return nil
}
