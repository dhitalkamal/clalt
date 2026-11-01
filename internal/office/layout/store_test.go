package layout

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// newStore is a store in a temp dir with one built-in template named loft and
// our own catalog. logs collects what the store logs.
func newStore(t *testing.T) (*Store, *[]string) {
	t.Helper()
	loft := valid()
	loft.Name, loft.Title = "loft", "Loft"
	raw, err := Encode(loft)
	if err != nil {
		t.Fatal(err)
	}
	var logs []string
	s := NewStore(t.TempDir(), func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	s.Catalog = testCatalog()
	s.Builtin = fstest.MapFS{"loft.json": {Data: raw}, "README.md": {Data: []byte("x")}}
	return s, &logs
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Save(valid()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("mine")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Mine" || len(got.Furniture) != 2 {
		t.Errorf("loaded %+v", got)
	}
	if st, err := os.Stat(filepath.Join(s.Root, "layouts", "mine.json")); err != nil || st.Mode().Perm()&0o077 != 0 {
		t.Errorf("file %v mode %v", err, st)
	}
}

func TestSaveRefusesAnInvalidLayoutAndWritesNothing(t *testing.T) {
	s, _ := newStore(t)
	l := valid()
	l.Rooms[1].X = 4
	if err := s.Save(l); err == nil {
		t.Fatal("saved an invalid layout")
	}
	if ents, _ := os.ReadDir(filepath.Join(s.Root, "layouts")); len(ents) != 0 {
		t.Errorf("left files behind: %v", ents)
	}
}

func TestBuiltInNamesCannotBeOverwrittenOrDeleted(t *testing.T) {
	s, _ := newStore(t)
	l := valid()
	l.Name = "loft"
	if err := s.Save(l); !errors.Is(err, ErrBuiltin) {
		t.Errorf("save over built-in = %v", err)
	}
	if err := s.Delete("loft"); !errors.Is(err, ErrBuiltin) {
		t.Errorf("delete built-in = %v", err)
	}
	if !s.IsBuiltin("loft") || s.IsBuiltin("mine") {
		t.Error("IsBuiltin is wrong")
	}
}

func TestAUserCopyShadowsABuiltIn(t *testing.T) {
	s, _ := newStore(t)
	l := valid()
	l.Name, l.Title = "loft", "My loft"
	raw, _ := Encode(l)
	if err := os.MkdirAll(filepath.Join(s.Root, "layouts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "layouts", "loft.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("loft")
	if err != nil || got.Title != "My loft" {
		t.Fatalf("got %q, %v", got.Title, err)
	}
	if err := s.Delete("loft"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load("loft"); got.Title != "Loft" {
		t.Errorf("after deleting the copy the built-in shows again, got %q", got.Title)
	}
}

func TestDeleteOnlyUserLayouts(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Save(valid()); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("mine"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("mine"); !errors.Is(err, ErrNotFound) {
		t.Errorf("load after delete = %v", err)
	}
	if err := s.Delete("mine"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete = %v", err)
	}
	if err := s.Delete("../x"); err == nil {
		t.Error("deleted a path")
	}
}

func TestListHasBuiltInsAndUserLayoutsSkippingBadFiles(t *testing.T) {
	s, logs := newStore(t)
	if err := s.Save(valid()); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Root, "layouts")
	for name, body := range map[string]string{"bad.json": "nope", "Not A Name.json": "{}", "notes.txt": "x"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	infos, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, i := range infos {
		got = append(got, fmt.Sprintf("%s/%s/%v", i.Name, i.Title, i.Builtin))
	}
	if strings.Join(got, ",") != "loft/Loft/true,mine/Mine/false" {
		t.Errorf("list = %v", got)
	}
	if len(*logs) == 0 {
		t.Error("the bad file was skipped without a log line")
	}
}

func TestLoadRefusesAFileWhoseNameIsNotItsFileName(t *testing.T) {
	s, _ := newStore(t)
	raw, _ := Encode(valid())
	_ = os.MkdirAll(filepath.Join(s.Root, "layouts"), 0o700)
	_ = os.WriteFile(filepath.Join(s.Root, "layouts", "other.json"), raw, 0o600)
	if _, err := s.Load("other"); err == nil {
		t.Error("a file named other holding mine loaded")
	}
}

func TestLoadRefusesAnOversizeFile(t *testing.T) {
	s, _ := newStore(t)
	_ = os.MkdirAll(filepath.Join(s.Root, "layouts"), 0o700)
	big := strings.Repeat(" ", MaxBytes+1)
	_ = os.WriteFile(filepath.Join(s.Root, "layouts", "big.json"), []byte(big), 0o600)
	if _, err := s.Load("big"); err == nil {
		t.Error("loaded an oversize file")
	}
}

func TestActiveDefaultsWhenNothingIsSet(t *testing.T) {
	s, logs := newStore(t)
	l := s.Active()
	if l.Name != "default" || Validate(l, Catalog{}) != nil {
		t.Errorf("active = %q", l.Name)
	}
	if len(*logs) != 0 {
		t.Errorf("a fresh start logged %v", *logs)
	}
}

func TestSetActiveThenActive(t *testing.T) {
	s, _ := newStore(t)
	if err := s.SetActive("loft"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveName() != "loft" || s.Active().Title != "Loft" {
		t.Errorf("active = %q", s.ActiveName())
	}
	if err := s.SetActive("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown name = %v", err)
	}
	if err := s.SetActive("../etc"); err == nil {
		t.Error("set a path as active")
	}
	if s.ActiveName() != "loft" {
		t.Error("a refused SetActive changed the active layout")
	}
}

func TestActiveFallsBackOnACorruptActiveFile(t *testing.T) {
	s, logs := newStore(t)
	_ = os.MkdirAll(s.Root, 0o700)
	for _, body := range []string{"{", `{"name":"../x"}`, `{"name":"x","extra":1}`, ``} {
		_ = os.WriteFile(filepath.Join(s.Root, "active.json"), []byte(body), 0o600)
		if l := s.Active(); l.Name != "default" {
			t.Errorf("%q: active = %q", body, l.Name)
		}
	}
	if len(*logs) == 0 {
		t.Error("no log line for a corrupt active.json")
	}
}

func TestActiveFallsBackOnACorruptLayoutFile(t *testing.T) {
	s, logs := newStore(t)
	if err := s.Save(valid()); err != nil {
		t.Fatal(err)
	}
	if err := s.SetActive("mine"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(s.Root, "layouts", "mine.json"), []byte(`{"v":1,`), 0o600)
	if l := s.Active(); l.Name != "default" {
		t.Errorf("active = %q, want the fallback", l.Name)
	}
	if len(*logs) == 0 {
		t.Error("no log line for a corrupt layout")
	}
}

func TestResetPointsAtTheDefaultAndKeepsFiles(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save(valid())
	_ = s.SetActive("mine")
	if err := s.SetActive(DefaultName); err != nil {
		t.Fatal(err)
	}
	if s.ActiveName() != DefaultName {
		t.Errorf("active = %q", s.ActiveName())
	}
	if _, err := s.Load("mine"); err != nil {
		t.Errorf("reset removed a layout: %v", err)
	}
}

func TestStoreWithoutARootStillServesTheDefault(t *testing.T) {
	s := NewStore("", nil)
	s.Catalog = testCatalog()
	s.Builtin = fstest.MapFS{}
	if l := s.Active(); l.Name != "default" {
		t.Errorf("active = %q", l.Name)
	}
	if err := s.Save(valid()); err == nil {
		t.Error("saved with no folder")
	}
}

func TestMinimalLayoutValidatesWithoutACatalog(t *testing.T) {
	if err := Validate(Minimal(), Catalog{}); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRefusesAnEmptyHallTitle(t *testing.T) {
	s, _ := newStore(t)
	for _, title := range []string{"", "   ", "\t"} {
		l := valid()
		l.Title = title
		err := s.Check(l)
		if err == nil || !strings.Contains(err.Error(), "title is empty") {
			t.Errorf("title %q: err = %v, want title is empty", title, err)
		}
	}
	if err := s.Check(valid()); err != nil {
		t.Errorf("valid layout: %v", err)
	}
}

func TestListLogsEachSkipOnce(t *testing.T) {
	s, logs := newStore(t)
	dir := filepath.Join(s.Root, "layouts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("bad.json", "nope")
	write("Not A Name.json", "{}")
	for i := 0; i < 3; i++ {
		if _, err := s.List(); err != nil {
			t.Fatal(err)
		}
	}
	if len(*logs) != 2 {
		t.Fatalf("logs after 3 polls = %d, want 2: %v", len(*logs), *logs)
	}
	// a different problem in the same file is a new line
	write("bad.json", `{"v":1}`)
	if _, err := s.List(); err != nil {
		t.Fatal(err)
	}
	if len(*logs) != 3 {
		t.Fatalf("logs after the file changed = %d, want 3: %v", len(*logs), *logs)
	}
}
