package crew

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// pins are what config.local.json of a gateway home is given: Claude as the
// backend of the agent and of the crew members, the cheapest model for
// background and subagent work, and tool calls that ask first, since what
// "auto" approves is not documented. docs/plans/crew.md says where each comes
// from. config.local.json is the overlay whose values win over config.json, so
// they hold whatever the Kiro dashboard writes.
var pins = map[string]any{
	"acp_backend":        "claude",
	"member_acp_backend": "claude",
	"role_models":        map[string]any{"background": "haiku", "subagent": "haiku"},
	"approval_mode":      "interactive",
}

// pinDefaults writes the pins into config.local.json of the home, keeping every
// other key of the file. kirocrew config set is not used: the file is Kiro
// Crew's documented overlay and clalt owns this one. a file that is not a json
// object is an error and is left as it is. the write is atomic and private.
func pinDefaults(p Paths) error {
	cfg := map[string]any{}
	raw, err := fsutil.ReadFile(p.LocalConfig)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(raw, &cfg); err != nil || cfg == nil {
			return fmt.Errorf("%s is not a json object, fix or remove it: %v", p.LocalConfig, err)
		}
	}
	agent, _ := cfg["agent"].(map[string]any)
	if agent == nil {
		agent = map[string]any{}
	}
	for k, v := range pins {
		if sub, ok := v.(map[string]any); ok {
			merged, _ := agent[k].(map[string]any)
			if merged == nil {
				merged = map[string]any{}
			}
			for sk, sv := range sub {
				merged[sk] = sv
			}
			v = merged
		}
		agent[k] = v
	}
	cfg["agent"] = agent
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p.LocalConfig, append(out, '\n'), 0o600)
}

// reason is what a program said was wrong, or else how it ended.
func reason(err error, stderr []byte) string {
	if line := firstLine(string(stderr)); line != "" {
		return line
	}
	return err.Error()
}

// Accept refuses a version that is not supported, unless the check was lifted.
func (m *Manager) Accept(version string) error {
	if m.o.AllowUntested {
		return nil
	}
	return CheckVersion(version)
}

// version asks kirocrew which version it is and refuses an unsupported one.
func (m *Manager) version(ctx context.Context, bin string, env []string, p Paths) (string, error) {
	stdout, stderr, err := m.o.Runner.Output(ctx, Spec{Bin: bin, Args: []string{"--version"}, Env: env, Dir: p.Home})
	if err != nil {
		return "", fmt.Errorf("cannot ask kirocrew for its version: %s", reason(err, stderr))
	}
	version, err := ParseVersion(strings.TrimSpace(string(stdout)))
	if err != nil {
		return "", err
	}
	return version, m.Accept(version)
}

// InstalledVersion asks the installed kirocrew for its version, without a
// gateway home: it is for doctor, which only wants to know. the home it is given
// is inside clalt's crew folder, and kirocrew has not been seen to touch it.
func (m *Manager) InstalledVersion(ctx context.Context) (string, error) {
	bin, err := m.o.Binary()
	if err != nil {
		return "", err
	}
	env := ProbeEnv(m.o.Environ(), filepath.Join(Root(), ".probe"))
	stdout, stderr, err := m.o.Runner.Output(ctx, Spec{Bin: bin, Args: []string{"--version"}, Env: env})
	if err != nil {
		return "", fmt.Errorf("cannot ask kirocrew for its version: %s", reason(err, stderr))
	}
	return ParseVersion(string(stdout))
}
