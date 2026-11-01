package reviews

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// maxRepos bounds the opt-in list.
const maxRepos = 100

// Config is what Settings stores: off until turned on, and only for the repos
// that were opted in one by one.
type Config struct {
	Enabled bool     `json:"enabled"`
	Repos   []string `json:"repos"`
}

const configName = "config.json"

// LoadConfig reads dir/config.json. no file is the default (off). a file that
// cannot be read gives the default and an error.
func LoadConfig(dir string) (Config, error) {
	raw, err := fsutil.ReadFile(filepath.Join(dir, configName))
	if errors.Is(err, fs.ErrNotExist) {
		return Config{Repos: []string{}}, nil
	}
	if err != nil {
		return Config{Repos: []string{}}, err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{Repos: []string{}}, fmt.Errorf("reviews: config.json is broken: %w", err)
	}
	c.Repos = cleanRepos(c.Repos)
	return c, nil
}

// cleanRepos keeps the valid names, once each (case-insensitive, first spelling wins), sorted.
func cleanRepos(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range in {
		key := strings.ToLower(r)
		if ValidRepo(r) && !seen[key] && len(out) < maxRepos {
			seen[key] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

// SaveConfig validates and writes the config with mode 0600.
func SaveConfig(dir string, c Config) error {
	for _, r := range c.Repos {
		if !ValidRepo(r) {
			return fmt.Errorf("reviews: %q is not a repository (owner/name)", r)
		}
	}
	if len(c.Repos) > maxRepos {
		return fmt.Errorf("reviews: at most %d repositories", maxRepos)
	}
	c.Repos = cleanRepos(c.Repos)
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(dir, configName), raw, 0o600)
}
