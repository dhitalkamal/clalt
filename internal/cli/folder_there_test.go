package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dhitalkamal/clalt/internal/config"
)

// only "not there" means absent. whatever else stops clalt from looking is an
// error, so that a folder it cannot see is never taken for one without a login
// or for a free name.
func TestFolderThere(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(dir, "folder"))
	mustSymlink(t, filepath.Join(dir, "nowhere"), filepath.Join(dir, "dangling"))

	cases := []struct {
		name    string
		path    string
		there   bool
		wantErr bool
	}{
		{"a folder", filepath.Join(dir, "folder"), true, false},
		{"a file", file, true, false},
		{"a link to nowhere is an entry", filepath.Join(dir, "dangling"), true, false},
		{"nothing", filepath.Join(dir, "none"), false, false},
	}
	// windows reports a path below a file as not found, same as a missing one
	if runtime.GOOS != "windows" {
		cases = append(cases, struct {
			name    string
			path    string
			there   bool
			wantErr bool
		}{"below a file", filepath.Join(file, "below"), false, true})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			there, err := folderThere(c.path)
			if there != c.there || (err != nil) != c.wantErr {
				t.Errorf("folderThere = %v, %v; want %v, error %v", there, err, c.there, c.wantErr)
			}
			if errors.Is(err, fs.ErrNotExist) {
				t.Errorf("err = %v: an absent folder is not an error", err)
			}
		})
	}
}

func TestFolderThereWhenTheParentCannotBeLookedInto(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "inside"))
	if !makeUnlookable(t, dir) {
		t.Skip("no permission bits to take away")
	}

	there, err := folderThere(filepath.Join(dir, "inside"))

	if err == nil || there {
		t.Errorf("folderThere = %v, %v: a folder that cannot be seen is unknown", there, err)
	}
}

func TestFolderExistsLooksAtTheFolderOfTheId(t *testing.T) {
	newEnv(t)
	mustMkdir(t, accountDir(t, "taken"))

	if got, err := folderExists("taken"); err != nil || !got {
		t.Errorf("a folder that is there: %v, %v", got, err)
	}
	if got, err := folderExists("free"); err != nil || got {
		t.Errorf("a name nobody has: %v, %v", got, err)
	}
	for _, id := range []string{"", "..", "a/b", "con"} {
		if _, err := folderExists(id); err == nil {
			t.Errorf("folderExists(%q) accepted an id that cannot be a folder name", id)
		}
	}
}

func TestFolderExistsIsAnErrorWhenTheAccountsFolderCannotBeLookedInto(t *testing.T) {
	newEnv(t)
	mustMkdir(t, accountDir(t, "work"))
	if !makeUnlookable(t, config.AccountsDir()) {
		t.Skip("no permission bits to take away")
	}

	got, err := folderExists("work")

	if err == nil || got {
		t.Errorf("folderExists = %v, %v: a folder that cannot be seen is not a free name", got, err)
	}
}
