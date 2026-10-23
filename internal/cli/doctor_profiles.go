package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/migrate"
)

// profilesCheck reports the old profile folders for as long as one exists.
// v0.1.0 staged a plaintext login in some of them and kept a second copy of the
// history in all of them. the folders and the login files are looked up in
// clalt's own folder. only the history needs the shared home, so a missing
// HOME or a run session limits that part and nothing else.
func (a *App) profilesCheck() checkResult {
	folders, err := oldProfileFolders()
	switch {
	case err != nil:
		return result("leftover", "oldProfiles", statusWarn, "could not check the old profile folders: %v", err)
	case len(folders) == 0:
		return result("leftover", "oldProfiles", statusOK, "no old profile folders")
	}
	parts := []string{fmt.Sprintf("%s in %s", count(len(folders), "old profile folder"), config.ProfilesDir())}
	logins, unknown, why := loginFilesIn(folders)
	if len(logins) > 0 {
		parts = append(parts, "plaintext logins from v0.1.0 are in .credentials.json files here (clalt never opens them): "+
			strings.Join(logins, ", ")+". delete those files")
	}
	if len(unknown) > 0 {
		parts = append(parts, fmt.Sprintf("could not look for a login file in %s: %v", strings.Join(unknown, ", "), why))
	}
	parts = append(parts, oldHistory())
	return result("leftover", "oldProfiles", statusWarn, "%s", strings.Join(parts, ". "))
}

// oldProfileFolders lists what is directly under profiles/ and counts as a
// profile the way migrate counts it: a folder, or a link that leads to one or
// to nothing. a plain file is not a profile.
func oldProfileFolders() ([]string, error) {
	dir := config.ProfilesDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		switch {
		case e.IsDir():
			names = append(names, e.Name())
		case e.Type()&fs.ModeSymlink != 0:
			if st, err := os.Stat(filepath.Join(dir, e.Name())); err != nil || st.IsDir() {
				names = append(names, e.Name())
			}
		}
	}
	return names, nil
}

// loginFilesIn finds the .credentials.json files in the profile folders. only
// whether one exists is asked, it is never opened. a folder that cannot be
// looked into may hold one, so it is listed apart with the first reason.
func loginFilesIn(folders []string) (found, unknown []string, why error) {
	for _, name := range folders {
		path := filepath.Join(config.ProfilesDir(), name, ".credentials.json")
		there, err := folderThere(path)
		switch {
		case err != nil:
			unknown = append(unknown, name)
			if why == nil {
				why = err
			}
		case there:
			found = append(found, path)
		}
	}
	return found, unknown, why
}

// oldHistory says whether the history in the old profile folders is copied. a
// migrate dry run answers that and writes nothing.
func oldHistory() string {
	if config.ClaudeProjectsDir() == "" {
		return "history not checked: " + noHomeKnown
	}
	dest, err := migrateDest()
	if err != nil {
		return "history not checked inside a 'clalt run' session. run 'clalt doctor' from a normal shell"
	}
	rep, err := migrate.Run(config.ProfilesDir(), dest, migrate.Options{DryRun: true})
	switch {
	case err != nil:
		return fmt.Sprintf("history not checked: %v", err)
	case rep.Total.Copied+rep.Total.Failed == 0 && rep.Total.Conflicts > 0:
		return "all history is already in the shared folder; " + conflictsLeft(rep)
	case rep.Total.Copied+rep.Total.Conflicts+rep.Total.Failed > 0:
		return "history not copied yet: " + tally(rep.Total, true) +
			". run 'clalt migrate --dry-run' to see it, 'clalt migrate' to copy it"
	}
	return "the history in them is already copied, so the folders can go when you like"
}

// conflictsLeft names the files that differ from the shared copy and so stay
// only in the profile folders. a long list is cut short.
func conflictsLeft(rep migrate.Report) string {
	var paths []string
	for _, p := range rep.Profiles {
		for _, c := range p.Conflicts {
			paths = append(paths, filepath.Join(rep.Source, p.Name, "projects", c.Path))
		}
	}
	verb := "differs and stays"
	if len(paths) != 1 {
		verb = "differ and stay"
	}
	shown := paths
	if len(shown) > 3 {
		shown = shown[:3]
	}
	list := strings.Join(shown, ", ")
	if len(paths) > len(shown) {
		list += fmt.Sprintf(", and %d more", len(paths)-len(shown))
	}
	return fmt.Sprintf("%s %s only in the profile folder: %s", count(len(paths), "file"), verb, list)
}
