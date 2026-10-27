package reviews

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// Store keeps one JSON file per review in Dir, owner only.
type Store struct{ Dir string }

// Save writes the review, replacing an older file of the same id.
func (s Store) Save(r Review) error {
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if !idRe.MatchString(r.ID()) {
		return errors.New("reviews: not a storable review id: " + r.ID())
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(s.Dir, r.ID()+".json"), raw, 0o600)
}

// Get reads one review. an id that is not an id is simply not found, so a
// path can never be built from it.
func (s Store) Get(id string) (Review, bool, error) {
	if !idRe.MatchString(id) {
		return Review{}, false, nil
	}
	raw, err := fsutil.ReadFile(filepath.Join(s.Dir, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Review{}, false, nil
	}
	if err != nil {
		return Review{}, false, err
	}
	var r Review
	if err := json.Unmarshal(raw, &r); err != nil {
		return Review{}, false, nil
	}
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	return r, true, nil
}

// List reads every review, newest first. files that are not reviews (the
// config) or are broken are left out.
func (s Store) List() ([]Review, error) {
	ents, err := os.ReadDir(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Review{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Review{}
	for _, e := range ents {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !idRe.MatchString(id) {
			continue
		}
		if r, ok, err := s.Get(id); err == nil && ok {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].EndedAt.Equal(out[j].EndedAt) {
			return out[i].EndedAt.After(out[j].EndedAt)
		}
		return out[i].ID() < out[j].ID()
	})
	return out, nil
}

// Has says whether this commit of this pull request has a stored result.
func (s Store) Has(repo string, number int, sha string) bool {
	if len(sha) < 7 {
		return false
	}
	_, err := os.Stat(filepath.Join(s.Dir, idOf(repo, number, sha)+".json"))
	return err == nil
}
