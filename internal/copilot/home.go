// Package copilot reads the session files GitHub Copilot CLI already writes,
// so the office can show its sessions. it only reads
// session-state/<id>/events.jsonl and workspace.yaml, never Copilot's login or
// settings files, and it keeps no free text: only ids, cwd, state, model and
// times.
package copilot

import (
	"os"
	"path/filepath"

	"github.com/dhitalkamal/clalt/internal/config"
)

// Home is Copilot's folder: $COPILOT_HOME, else ~/.copilot. empty when there
// is no home at all.
func Home() string {
	if d := os.Getenv("COPILOT_HOME"); d != "" {
		return d
	}
	h := config.Home()
	if h == "" {
		return ""
	}
	return filepath.Join(h, ".copilot")
}
