package copilot

import (
	"io"
	"os"
	"strings"
	"time"
)

// maxWorkspaceBytes bounds how much of workspace.yaml is read.
const maxWorkspaceBytes = 16 << 10

// workspace is the two things clalt takes from workspace.yaml.
type workspace struct {
	Cwd       string
	CreatedAt time.Time
}

// readWorkspace reads the flat top-level "key: value" lines it needs. a
// missing or odd file is an empty result. nested lines (indented) are skipped.
func readWorkspace(path string) workspace {
	var w workspace
	f, err := os.Open(path)
	if err != nil {
		return w
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, maxWorkspaceBytes))
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = unquote(strings.TrimSpace(v))
		switch k {
		case "cwd":
			w.Cwd = v
		case "created_at":
			w.CreatedAt, _ = time.Parse(time.RFC3339, v)
		}
	}
	return w
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '\'' || v[0] == '"') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}
