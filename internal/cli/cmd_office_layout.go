package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/fsutil"
	"github.com/dhitalkamal/clalt/internal/office/layout"
)

const officeLayoutUsage = "clalt office layout list | use <name> | export <name> [file] [--force] | import <file> [--name n] [--force] | reset"

func officeLayoutStore() *layout.Store {
	return layout.NewStore(config.OfficeDir(), officeLogf(dashboardLogger(officeLogPath())))
}

// cmdOfficeLayout manages the floor plans. none of it needs a running office.
func (a *App) cmdOfficeLayout(args []string) error {
	if len(args) == 0 {
		return usageError{"layout needs a subcommand", officeLayoutUsage}
	}
	store := officeLayoutStore()
	rest := args[1:]
	switch args[0] {
	case "list":
		if len(rest) > 0 {
			return usageError{"list takes no arguments", officeLayoutUsage}
		}
		return a.layoutList(store)
	case "use":
		if len(rest) != 1 {
			return usageError{"use needs one layout name", officeLayoutUsage}
		}
		if err := store.SetActive(rest[0]); err != nil {
			return layoutErr(rest[0], err)
		}
		fmt.Fprintf(a.Out, "active layout is now %s. a running office shows it after a reload.\n", rest[0])
		return nil
	case "reset":
		if len(rest) > 0 {
			return usageError{"reset takes no arguments", officeLayoutUsage}
		}
		if err := store.SetActive(layout.DefaultActive); err != nil {
			return err
		}
		fmt.Fprintln(a.Out, "active layout is the default again (rows). no layout file was deleted.")
		return nil
	case "export":
		return a.layoutExport(store, rest)
	case "import":
		return a.layoutImport(store, rest)
	}
	return usageError{"unknown layout subcommand " + args[0], officeLayoutUsage}
}

// layoutErr words a store error for a layout name.
func layoutErr(name string, err error) error {
	if errors.Is(err, layout.ErrNotFound) {
		return fmt.Errorf("no layout named %s (see clalt office layout list)", name)
	}
	return err
}

func (a *App) layoutList(store *layout.Store) error {
	infos, err := store.List()
	if err != nil {
		return err
	}
	active := store.ActiveName()
	for _, i := range infos {
		mark, kind := " ", "yours"
		if i.Name == active {
			mark = "*"
		}
		if i.Builtin {
			kind = "built-in"
		}
		fmt.Fprintf(a.Out, "%s %-24s %-9s %s\n", mark, i.Name, kind, i.Title)
	}
	for _, t := range layout.Templates {
		mark := " "
		if t.ID == active {
			mark = "*"
		}
		fmt.Fprintf(a.Out, "%s %-24s %-9s %s\n", mark, t.ID, "template", t.Title)
	}
	return nil
}

// layoutFlags splits the arguments of export and import into positionals and
// the flags they know.
func layoutFlags(args []string, withName bool) (pos []string, name string, force bool, err error) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--force":
			force = true
		case args[i] == "--name" && withName:
			if i++; i >= len(args) {
				return nil, "", false, usageError{"--name needs a layout name", officeLayoutUsage}
			}
			name = args[i]
		case strings.HasPrefix(args[i], "-"):
			return nil, "", false, usageError{"unknown flag " + args[i], officeLayoutUsage}
		default:
			pos = append(pos, args[i])
		}
	}
	return pos, name, force, nil
}

func (a *App) layoutExport(store *layout.Store, args []string) error {
	pos, _, force, err := layoutFlags(args, false)
	if err != nil {
		return err
	}
	if len(pos) < 1 || len(pos) > 2 {
		return usageError{"export needs a layout name and optionally a file", officeLayoutUsage}
	}
	l, err := store.Load(pos[0])
	if err != nil {
		return layoutErr(pos[0], err)
	}
	raw, err := layout.Encode(l)
	if err != nil {
		return err
	}
	if len(pos) == 1 {
		_, err = a.Out.Write(raw)
		return err
	}
	if force {
		err = fsutil.WriteFileAtomic(pos[1], raw, 0o644)
	} else {
		err = writeNew(pos[1], raw)
	}
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s exists, pass --force to overwrite it", pos[1])
	}
	if err == nil {
		fmt.Fprintf(a.Out, "wrote %s\n", pos[1])
	}
	return err
}

// writeNew creates a file that must not exist yet.
func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// layoutImport validates a file like any other untrusted input and stores it
// as a user layout. it does not make it active.
func (a *App) layoutImport(store *layout.Store, args []string) error {
	pos, name, force, err := layoutFlags(args, true)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageError{"import needs one file", officeLayoutUsage}
	}
	st, err := os.Stat(pos[0])
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() > layout.MaxBytes {
		return fmt.Errorf("%s is not a regular file of at most %d KB", pos[0], layout.MaxBytes/1024)
	}
	raw, err := fsutil.ReadFile(pos[0])
	if err != nil {
		return err
	}
	l, err := layout.Decode(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", pos[0], err)
	}
	if name != "" {
		l.Name = name
	}
	if err := store.Check(l); err != nil {
		return fmt.Errorf("%s is refused: %w", pos[0], err)
	}
	if store.IsBuiltin(l.Name) {
		return fmt.Errorf("%s is the name of a built-in template, pick another with --name", l.Name)
	}
	if _, err := store.Load(l.Name); err == nil && !force {
		return fmt.Errorf("a layout named %s exists, pass --force to replace it or --name to pick another", l.Name)
	}
	if err := store.Save(l); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "imported %s. use it with: clalt office layout use %s\n", l.Name, l.Name)
	return nil
}
