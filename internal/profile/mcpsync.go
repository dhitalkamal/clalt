package profile

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// mcpManifest records, inside the account folder, which server names clalt
// mirrored from the shared config. without it a server that later leaves the
// shared config could not be told from one the account added itself.
const mcpManifest = ".clalt-mcp.json"

// MCPChange lists server names by what a sync did or would do. names only:
// definitions can hold secrets (env blocks) and are never returned, printed or
// put in an error. Overwrote is the part of Updated that replaced a server the
// account had added itself, which a caller will want to tell the user.
type MCPChange struct {
	Added     []string
	Updated   []string
	Removed   []string
	Overwrote []string
}

// Empty is true when nothing was, or would be, added, updated or removed.
func (c MCPChange) Empty() bool { return len(c.Added)+len(c.Updated)+len(c.Removed) == 0 }

// seams for tests: afterPlan runs between planning and writing, where claude
// could change the account's file, and writeFile is how files are written.
var (
	afterPlan = func() {}
	writeFile = fsutil.WriteFileAtomic
)

type manifestFile struct {
	Synced []string `json:"synced"`
}

type mcpPlan struct {
	change   MCPChange
	config   []byte // new account .claude.json, nil when it stays as it is
	manifest []byte // new manifest, nil when it stays as it is
	seen     []byte // the account's .claude.json as the plan read it, nil when missing
}

// mcpTries is how often a sync plans again when claude rewrites the account's
// file between the plan and the write.
const mcpTries = 3

// SyncMCP mirrors the shared config's top-level mcpServers into the account's
// .claude.json. a shared server is added, or replaces the account's server of
// the same name; one clalt mirrored earlier that left the shared config is
// removed; a server the account added itself is kept. every other key of the
// account's file is carried over as is. a shared config that cannot be read or
// parsed aborts before anything is written, and so does one that is missing
// while servers were mirrored from it earlier: a half-written or absent file
// must not wipe the account's servers.
//
// the account's lock is held throughout, and the account's file is read again
// right before it is written. if claude changed it in between, the sync plans
// again, up to a few times. when the manifest cannot be written after the
// config was, the change is returned together with the error.
func SyncMCP(profileDir, sharedClaudeJSON string) (MCPChange, error) {
	if err := checkMCPPaths(profileDir, sharedClaudeJSON); err != nil {
		return MCPChange{}, err
	}
	unlock, err := lockAccount(profileDir)
	if err != nil {
		return MCPChange{}, err
	}
	defer unlock()
	configPath := filepath.Join(profileDir, ".claude.json")
	for try := 0; try < mcpTries; try++ {
		p, err := planMCP(profileDir, sharedClaudeJSON)
		if err != nil {
			return MCPChange{}, err
		}
		if p.config == nil && p.manifest == nil {
			return p.change, nil
		}
		afterPlan()
		// the config goes first: if the second write never happens, the next run
		// sees an old manifest next to a finished config and just repairs it
		if p.config != nil {
			unchanged, err := unchangedSince(configPath, p.seen)
			if err != nil {
				return MCPChange{}, err
			}
			if !unchanged {
				continue
			}
			if err := writeFile(configPath, p.config, 0o600); err != nil {
				return MCPChange{}, err
			}
		}
		if p.manifest != nil {
			if err := writeFile(filepath.Join(profileDir, mcpManifest), p.manifest, 0o600); err != nil {
				return p.change, fmt.Errorf("the servers were updated, but writing %s failed: %w", mcpManifest, err)
			}
		}
		return p.change, nil
	}
	return MCPChange{}, fmt.Errorf("%s kept changing while it was being synced, try again", configPath)
}

// PlanMCP is SyncMCP without the writes, for the health check. it takes no
// lock, so it never waits for a sync.
func PlanMCP(profileDir, sharedClaudeJSON string) (MCPChange, error) {
	if err := checkMCPPaths(profileDir, sharedClaudeJSON); err != nil {
		return MCPChange{}, err
	}
	p, err := planMCP(profileDir, sharedClaudeJSON)
	return p.change, err
}

func planMCP(profileDir, sharedPath string) (mcpPlan, error) {
	configPath := filepath.Join(profileDir, ".claude.json")

	sharedRaw, sharedTop, err := readObject(sharedPath)
	if err != nil {
		return mcpPlan{}, err
	}
	shared, err := serversOf(sharedTop, sharedPath)
	if err != nil {
		return mcpPlan{}, err
	}
	prev, err := readManifest(filepath.Join(profileDir, mcpManifest))
	if err != nil {
		return mcpPlan{}, err
	}
	owned := map[string]bool{}
	for _, name := range prev {
		owned[name] = true
	}
	// a missing file is not "no servers": the home may be unmounted, and acting
	// on that would remove every server clalt mirrored
	if sharedRaw == nil && len(owned) > 0 {
		return mcpPlan{}, fmt.Errorf("%s is missing, but %d servers were mirrored from it earlier; not removing them", sharedPath, len(owned))
	}
	seen, top, err := readObject(configPath)
	if err != nil {
		return mcpPlan{}, err
	}
	servers, err := serversOf(top, configPath)
	if err != nil {
		return mcpPlan{}, err
	}

	p := mcpPlan{seen: seen}
	for name, def := range shared {
		cur, there := servers[name]
		switch {
		case !there:
			p.change.Added = append(p.change.Added, name)
		case !sameJSON(cur, def):
			p.change.Updated = append(p.change.Updated, name)
			if !owned[name] {
				p.change.Overwrote = append(p.change.Overwrote, name)
			}
		default:
			continue
		}
		servers[name] = def
	}
	for _, name := range prev {
		if _, still := shared[name]; still {
			continue
		}
		if _, there := servers[name]; there {
			p.change.Removed = append(p.change.Removed, name)
			delete(servers, name)
		}
	}
	sort.Strings(p.change.Added)
	sort.Strings(p.change.Updated)
	sort.Strings(p.change.Removed)
	sort.Strings(p.change.Overwrote)

	if !p.change.Empty() {
		if err := refuseSymlink(configPath); err != nil {
			return mcpPlan{}, err
		}
		raw, err := encodeJSON(servers, "")
		if err != nil {
			return mcpPlan{}, err
		}
		top["mcpServers"] = bytes.TrimSpace(raw)
		if p.config, err = encodeJSON(top, "  "); err != nil {
			return mcpPlan{}, err
		}
	}
	// every shared name now exists in the account, so the shared names are
	// exactly what clalt manages there
	names := sortedSet(keys(shared))
	if !reflect.DeepEqual(names, sortedSet(prev)) {
		if p.manifest, err = encodeJSON(manifestFile{Synced: names}, "  "); err != nil {
			return mcpPlan{}, err
		}
	}
	return p, nil
}

// refuseSymlink stops a write that would go through a link: the atomic write
// would replace the link with a file, and what it led to would go stale.
func refuseSymlink(path string) error {
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink, not writing through it", path)
	}
	return nil
}

// unchangedSince reports whether the file still holds the bytes a plan was made
// from (nil for a file that was missing).
func unchangedSince(path string, seen []byte) (bool, error) {
	cur, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return seen == nil, nil
	}
	if err != nil {
		return false, err
	}
	return seen != nil && bytes.Equal(cur, seen), nil
}
