package live

import (
	"regexp"
	"strings"
)

var (
	modelSuffix = regexp.MustCompile(`(\[[^\]]*\])$|-\d{8}$`)
	modelParts  = regexp.MustCompile(`^claude-(?:(\d+(?:-\d)?)-)?(opus|sonnet|haiku|fable|mythos)(?:-(\d+(?:-\d)?))?$`)
)

// modelName writes a model id the way people say it: claude-opus-5-5-2026...
// is "Opus 5.5". an id it does not know is shown as it is.
func modelName(id string) string {
	if id == "" {
		return "unknown"
	}
	base := modelSuffix.ReplaceAllString(modelSuffix.ReplaceAllString(id, ""), "")
	m := modelParts.FindStringSubmatch(base)
	if m == nil {
		return id
	}
	version := m[1]
	if version == "" {
		version = m[3]
	}
	family := strings.ToUpper(m[2][:1]) + m[2][1:]
	if version == "" {
		return family
	}
	return family + " " + strings.Replace(version, "-", ".", 1)
}
