package conversation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Options narrows List. Limit 0 means all.
type Options struct {
	Words []string
	Limit int
}

// ErrNotFound is returned by Find when no conversation has the id.
var ErrNotFound = errors.New("no conversation with that id")

// AmbiguousError is returned by Find when more than one conversation matches.
type AmbiguousError struct {
	Prefix  string
	Matches []Conversation
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%d conversations start with %q", len(e.Matches), e.Prefix)
}

var fileName = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\.jsonl$`)

// file is a transcript that has been found and not yet read.
type file struct {
	path, project, id string
	mtime             time.Time
}

// scan finds the transcripts directly inside the project folders of root
// without reading any. a root that is not there holds none, one that cannot be
// read is an error, and a project folder that cannot be read is skipped.
func scan(root string) ([]file, error) {
	projects, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []file
	for _, p := range projects {
		dir := filepath.Join(root, p.Name())
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.Type().IsRegular() || !fileName.MatchString(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, file{
				path: filepath.Join(dir, e.Name()), project: p.Name(),
				id: strings.ToLower(strings.TrimSuffix(e.Name(), ".jsonl")), mtime: info.ModTime(),
			})
		}
	}
	return out, nil
}

// List describes the conversations under root, newest activity first. files
// are taken newest first by modification time, which clalt migrate keeps, and
// read only until Limit of them match, so a short list is cheap. the result is
// then ordered by what the transcripts say.
func List(root string, o Options) ([]Conversation, error) {
	files, err := scan(root)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].mtime.Equal(files[j].mtime) {
			return files[i].mtime.After(files[j].mtime)
		}
		return files[i].path < files[j].path
	})
	words := make([]string, 0, len(o.Words))
	for _, w := range o.Words {
		words = append(words, strings.ToLower(w))
	}
	var out []Conversation
	for _, f := range files {
		c, ok, err := describe(f.path, f.project)
		if err != nil || !ok || !matches(c, words) {
			continue
		}
		out = append(out, c)
		if o.Limit > 0 && len(out) == o.Limit {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActive.After(out[j].LastActive) })
	return out, nil
}

func matches(c Conversation, words []string) bool {
	hay := strings.ToLower(strings.Join([]string{c.ID, c.Title, c.Cwd, c.Model}, "\n"))
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// Find returns the conversation whose id starts with prefix. a prefix that fits
// several, a copy of one session in two projects included, is an
// *AmbiguousError.
func Find(root, prefix string) (Conversation, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return Conversation{}, ErrNotFound
	}
	files, err := scan(root)
	if err != nil {
		return Conversation{}, err
	}
	var found []Conversation
	for _, f := range files {
		if !strings.HasPrefix(f.id, prefix) {
			continue
		}
		if c, ok, err := describe(f.path, f.project); err == nil && ok {
			found = append(found, c)
		}
	}
	switch len(found) {
	case 0:
		return Conversation{}, ErrNotFound
	case 1:
		return found[0], nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].LastActive.After(found[j].LastActive) })
	return Conversation{}, &AmbiguousError{Prefix: prefix, Matches: found}
}
