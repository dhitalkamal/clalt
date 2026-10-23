package migrate

import (
	"io/fs"
	"os"
	"path/filepath"
)

// walk copies every regular file under root, one profile's projects folder.
// WalkDir does not follow links, and one inside a profile could lead anywhere.
func (c *copier) walk(p *Profile, root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			p.fail(KindFolder, relName(root, path), err)
			return nil
		}
		switch {
		case d.IsDir():
		case d.Type().IsRegular():
			c.file(p, root, path)
		default:
			// never opened, never followed: a fifo would block forever
			p.skip(relName(root, path), path, d.Type())
		}
		return nil
	})
}

// relName is how a path is shown in a report: slash separated, relative to the
// projects folder, with "projects" standing for the folder itself.
func relName(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return "projects"
	}
	return filepath.ToSlash(rel)
}

func (p *Profile) skip(rel, path string, mode fs.FileMode) {
	s := Skip{Path: rel, Kind: skipKind(mode)}
	if mode&fs.ModeSymlink != 0 {
		s.Target, _ = os.Readlink(path)
	}
	p.Skipped++
	p.SkippedEntries = append(p.SkippedEntries, s)
}

func skipKind(mode fs.FileMode) string {
	switch {
	case mode&fs.ModeSymlink != 0:
		return "link"
	case mode&fs.ModeNamedPipe != 0:
		return "pipe"
	case mode&fs.ModeSocket != 0:
		return "socket"
	case mode&fs.ModeDevice != 0:
		return "device"
	case mode&fs.ModeIrregular != 0:
		return "irregular"
	}
	return "special"
}
