package officed

import "strings"

// MissingTool is a tool reviews need that cannot be found, with the dirs
// that were searched besides PATH.
type MissingTool struct {
	Name string
	Dirs []string
}

// warnMissing prints one warning per missing tool. nil Missing prints nothing.
func (m Manager) warnMissing() {
	if m.Missing == nil {
		return
	}
	for _, t := range m.Missing() {
		m.printf("warning: %s not found on PATH or in %s; reviews will fail\n", t.Name, strings.Join(t.Dirs, ", "))
	}
}
