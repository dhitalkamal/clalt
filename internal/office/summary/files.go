package summary

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// default caps of one request: files opened and bytes read.
const (
	defaultMaxFiles = 1500
	defaultMaxBytes = 256 << 20
)

type candidate struct {
	path, project string
	size          int64
	mtime         time.Time
}

// candidates lists the transcripts under the roots changed at or after since,
// newest first. a transcript not touched since a time cannot hold lines from
// after it. unreadable entries are skipped.
func candidates(roots []string, since time.Time) []candidate {
	var out []candidate
	seen := map[string]bool{}
	for _, root := range roots {
		real, err := filepath.EvalSymlinks(root)
		if err != nil || seen[real] {
			continue
		}
		seen[real] = true
		_ = filepath.WalkDir(real, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			fi, err := d.Info()
			if err != nil || !fi.Mode().IsRegular() || fi.ModTime().Before(since) {
				return nil
			}
			out = append(out, candidate{path: path, project: projectOf(real, path), size: fi.Size(), mtime: fi.ModTime()})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].mtime.Equal(out[j].mtime) {
			return out[i].mtime.After(out[j].mtime)
		}
		return out[i].path < out[j].path
	})
	return out
}

// projectOf is the first folder under the root, the project folder of claude.
func projectOf(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	return first
}

// readAll reads candidates newest first until a cap is hit and says whether
// something was left out.
func readAll(cands []candidate, maxFiles int, maxBytes int64, w window) (*scan, bool) {
	s := newScan()
	var bytes int64
	files, truncated := 0, false
	for _, c := range cands {
		if files >= maxFiles || bytes+c.size > maxBytes {
			truncated = true
			continue
		}
		f, err := os.Open(c.path)
		if err != nil {
			continue
		}
		files++
		bytes += c.size
		s.readFile(f, c.project, w)
		f.Close()
	}
	return s, truncated
}
