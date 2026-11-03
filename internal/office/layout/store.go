package layout

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// DefaultName is the layout used when nothing valid is active.
const DefaultName = "default"

var (
	// ErrBuiltin is a write or delete aimed at a built-in template name.
	ErrBuiltin = errors.New("that name is a built-in template, which is read only")
	// ErrNotFound is a name with no layout.
	ErrNotFound = errors.New("no such layout")
)

// Info is one line of the layout list.
type Info struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Builtin bool   `json:"builtin"`
}

// Store keeps layouts under Root/layouts and the active name in
// Root/active.json. built-in templates come from the embed and are read only,
// a user file of the same name shadows one. an empty Root is a store with no
// disk: it only serves built-ins and the default.
type Store struct {
	Root string
	// Catalog is what layouts are validated against, nil means the embedded one.
	Catalog Catalog
	// Builtin is the folder of built-in templates, nil means the embedded one.
	Builtin fs.FS
	Logf    func(format string, args ...any)

	// skips holds the skip lines already logged, so a poll does not repeat them
	skipMu sync.Mutex
	skips  map[string]bool
}

// NewStore is a store at root. logf gets one line per layout it had to skip.
func NewStore(root string, logf func(string, ...any)) *Store {
	return &Store{Root: root, Logf: logf}
}

func (s *Store) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// logSkip logs a skipped file once per server run: the same line is not repeated on every poll, a new
// problem in the same file is a new line.
func (s *Store) logSkip(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	s.skipMu.Lock()
	seen := s.skips[line]
	if s.skips == nil {
		s.skips = map[string]bool{}
	}
	s.skips[line] = true
	s.skipMu.Unlock()
	if !seen {
		s.logf("%s", line)
	}
}

func (s *Store) catalog() Catalog {
	if s.Catalog != nil {
		return s.Catalog
	}
	cat, err := EmbeddedCatalog()
	if err != nil {
		s.logf("layout catalog: %v", err)
	}
	return cat
}

func (s *Store) builtinFS() fs.FS {
	if s.Builtin != nil {
		return s.Builtin
	}
	sub, err := fs.Sub(embeddedFS, "data/templates")
	if err != nil {
		return nil
	}
	return sub
}

func (s *Store) dir() string { return filepath.Join(s.Root, "layouts") }

func (s *Store) path(name string) string { return filepath.Join(s.dir(), name+".json") }

// IsBuiltin says whether a built-in template has that name.
func (s *Store) IsBuiltin(name string) bool {
	if fsys := s.builtinFS(); fsys != nil && ValidName(name) {
		st, err := fs.Stat(fsys, name+".json")
		return err == nil && st.Mode().IsRegular()
	}
	return false
}

// readUser reads one user file, refusing anything over the cap.
func (s *Store) readUser(name string) ([]byte, error) {
	if s.Root == "" {
		return nil, ErrNotFound
	}
	st, err := os.Stat(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > MaxBytes {
		return nil, fmt.Errorf("layout %s is not a regular file of at most %d KB", name, MaxBytes/1024)
	}
	return fsutil.ReadFile(s.path(name))
}

// StoredBytes is the user file of a layout as it is on disk, ErrNotFound when
// there is none. a built-in template has no user file until it is copied.
func (s *Store) StoredBytes(name string) ([]byte, error) {
	if !ValidName(name) {
		return nil, errors.New("not a layout name")
	}
	return s.readUser(name)
}

// parse decodes and validates a stored layout, which must carry the name it
// is stored under.
func (s *Store) parse(name string, raw []byte) (Layout, error) {
	l, err := Decode(raw)
	if err != nil {
		return Layout{}, err
	}
	if l.Name != name {
		return Layout{}, fmt.Errorf("layout %s holds a layout named %q", name, l.Name)
	}
	if l.Template != "" {
		return Layout{}, fmt.Errorf("layout %s names a template, which is never stored", name)
	}
	if err := Validate(l, s.catalog()); err != nil {
		return Layout{}, err
	}
	return l, nil
}

// Load reads a layout by name: the user file when there is one, else the
// built-in template. the default can always be loaded.
func (s *Store) Load(name string) (Layout, error) {
	if !ValidName(name) {
		return Layout{}, errors.New("not a layout name")
	}
	raw, err := s.readUser(name)
	if err == nil {
		return s.parse(name, raw)
	}
	if !errors.Is(err, ErrNotFound) {
		return Layout{}, err
	}
	if fsys := s.builtinFS(); fsys != nil && s.IsBuiltin(name) {
		raw, err := fs.ReadFile(fsys, name+".json")
		if err != nil {
			return Layout{}, err
		}
		return s.parse(name, raw)
	}
	if name == DefaultName {
		return Minimal(), nil
	}
	return Layout{}, ErrNotFound
}

// Save validates l and writes it as a user layout, atomically. a built-in
// name is refused.
func (s *Store) Save(l Layout) error {
	l = normalize(l)
	if err := Validate(l, s.catalog()); err != nil {
		return err
	}
	if l.Template != "" {
		return errors.New("a layout that names a template is generated, never stored")
	}
	if s.IsBuiltin(l.Name) || IsTemplate(l.Name) {
		return ErrBuiltin
	}
	if s.Root == "" {
		return errors.New("no layouts folder")
	}
	raw, err := Encode(l)
	if err != nil {
		return err
	}
	if len(raw) > MaxBytes {
		return fmt.Errorf("layout is larger than %d KB", MaxBytes/1024)
	}
	if err := os.MkdirAll(s.dir(), 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path(l.Name), raw, 0o600)
}

// Delete removes a user layout. a built-in template cannot be removed, and a
// user copy shadowing one can, which shows the template again.
func (s *Store) Delete(name string) error {
	if !ValidName(name) {
		return errors.New("not a layout name")
	}
	if s.Root != "" {
		err := os.Remove(s.path(name))
		if err == nil {
			return nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if s.IsBuiltin(name) || IsTemplate(name) {
		return ErrBuiltin
	}
	return ErrNotFound
}

// List is every built-in and user layout that loads, by name. a file that does
// not is logged and left out.
func (s *Store) List() ([]Info, error) {
	byName := map[string]Info{}
	if fsys := s.builtinFS(); fsys != nil {
		ents, _ := fs.ReadDir(fsys, ".")
		for _, e := range ents {
			s.addInfo(byName, strings.TrimSuffix(e.Name(), ".json"), e.Name(), true)
		}
	}
	if s.Root != "" {
		ents, err := os.ReadDir(s.dir())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		for _, e := range ents {
			s.addInfo(byName, strings.TrimSuffix(e.Name(), ".json"), e.Name(), false)
		}
	}
	out := make([]Info, 0, len(byName))
	for _, i := range byName {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

// addInfo adds one file of the user folder or the embed to the list.
func (s *Store) addInfo(into map[string]Info, name, file string, builtin bool) {
	if !strings.HasSuffix(file, ".json") {
		return
	}
	if !ValidName(name) {
		s.logSkip("layout file %q skipped: not a layout name", file)
		return
	}
	var l Layout
	var err error
	if builtin {
		var raw []byte
		if raw, err = fs.ReadFile(s.builtinFS(), file); err == nil {
			l, err = s.parse(name, raw)
		}
	} else {
		var raw []byte
		if raw, err = s.readUser(name); err == nil {
			l, err = s.parse(name, raw)
		}
	}
	if err != nil {
		s.logSkip("layout %s skipped: %v", name, err)
		return
	}
	into[name] = Info{Name: name, Title: l.Title, Builtin: builtin}
}
