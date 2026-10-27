package office

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// AgentDef is one custom agent file: the name and one line description of its
// front matter. Source is "user" or "project".
type AgentDef struct {
	Name   string
	Desc   string
	Source string
}

// TeamSource lists the custom agents for the working folders of the running
// sessions. cwds are the real folders, not cut to a name.
type TeamSource interface {
	Agents(cwds []string) []AgentDef
}

// limits of reading an agent folder.
const (
	maxAgentFile   = 64 * 1024
	maxAgentFiles  = 64
	maxProjectDirs = 64
	teamCacheTTL   = 30 * time.Second
)

type dirRead struct {
	at   time.Time
	defs []AgentDef
}

// TeamFiles reads <claude home>/agents and the .claude/agents folder of each
// session folder. every folder is re-read at most every 30 s. it only reads
// *.md files and only the name and description of their front matter.
type TeamFiles struct {
	userDir string
	now     func() time.Time
	mu      sync.Mutex
	cache   map[string]dirRead
}

// NewTeamFiles reads userDir (empty for none) and the project folders. now is
// the clock of the cache, time.Now when nil.
func NewTeamFiles(userDir string, now func() time.Time) *TeamFiles {
	if now == nil {
		now = time.Now
	}
	return &TeamFiles{userDir: userDir, now: now, cache: map[string]dirRead{}}
}

// Agents lists the user agents, then the project ones in cwd order.
func (t *TeamFiles) Agents(cwds []string) []AgentDef {
	t.mu.Lock()
	defer t.mu.Unlock()
	type job struct{ dir, source string }
	var jobs []job
	if t.userDir != "" {
		jobs = append(jobs, job{t.userDir, "user"})
	}
	seen := map[string]bool{}
	projects := 0
	for _, cwd := range cwds {
		if !filepath.IsAbs(cwd) || projects == maxProjectDirs {
			continue
		}
		dir := filepath.Join(cwd, ".claude", "agents")
		// a session in the home folder would list the user agents twice
		if seen[dir] || (t.userDir != "" && fsutil.SamePath(dir, t.userDir)) {
			continue
		}
		seen[dir] = true
		projects++
		jobs = append(jobs, job{dir, "project"})
	}
	now := t.now()
	keep := map[string]bool{}
	var out []AgentDef
	for _, j := range jobs {
		key := j.source + "\x00" + j.dir
		keep[key] = true
		c, ok := t.cache[key]
		if !ok || now.Sub(c.at) >= teamCacheTTL || now.Before(c.at) {
			c = dirRead{at: now, defs: readAgentDir(j.dir, j.source)}
			t.cache[key] = c
		}
		out = append(out, c.defs...)
	}
	for k := range t.cache {
		if !keep[k] {
			delete(t.cache, k)
		}
	}
	return out
}

// readAgentDir reads the agent files of one folder. a missing folder, an
// unreadable file or a file without a name gives nothing. only *.md names are
// looked at (backups such as x.md.pre-1 end otherwise), at most 64 of them,
// each at most 64 KB, and a symlink only counts when it stays in the folder.
func readAgentDir(dir, source string) []AgentDef {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	root := fsutil.ResolvePath(dir)
	var out []AgentDef
	files := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if files++; files > maxAgentFiles {
			break
		}
		path := filepath.Join(dir, e.Name())
		if e.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(path)
			if err != nil || !fsutil.Within(target, root) {
				continue
			}
			path = target
		}
		name, desc, ok := readFrontMatter(path)
		if ok {
			out = append(out, AgentDef{Name: name, Desc: desc, Source: source})
		}
	}
	return out
}

// readFrontMatter opens one file and returns its name and description. false
// when it is not a regular file, is too large, has no front matter or no name.
func readFrontMatter(path string) (name, desc string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", false
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() || fi.Size() > maxAgentFile {
		return "", "", false
	}
	return parseFrontMatter(io.LimitReader(f, maxAgentFile))
}

// parseFrontMatter reads the block between the first two --- lines and takes
// the name and description keys, nothing else. the body is never read: it
// stops at the closing line.
func parseFrontMatter(r io.Reader) (name, desc string, ok bool) {
	sc := bufio.NewScanner(r)
	var lines []string
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if len(lines) == 0 {
			if strings.TrimPrefix(line, "\ufeff") != "---" {
				return "", "", false
			}
			lines = append(lines, line)
			continue
		}
		if line == "---" {
			lines = lines[1:]
			return frontKeys(lines)
		}
		lines = append(lines, line)
	}
	return "", "", false
}

func frontKeys(lines []string) (name, desc string, ok bool) {
	for i := 0; i < len(lines); i++ {
		key, val, found := strings.Cut(lines[i], ":")
		if !found || (key != "name" && key != "description") {
			continue
		}
		val = strings.TrimSpace(val)
		// indented lines after the key belong to it (block or wrapped text)
		for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t")) {
			i++
			val += " " + strings.TrimSpace(lines[i])
		}
		val = scalar(val)
		if key == "name" {
			name = val
		} else {
			desc = val
		}
	}
	return name, desc, name != ""
}

// scalar drops the yaml block marker and the quotes of a value, turns a
// literal \n into a space (agent files often hold one), and collapses spaces.
func scalar(v string) string {
	if marker, rest, _ := strings.Cut(v, " "); blockMark.MatchString(marker) {
		v = rest
	} else if n := len(v); n >= 2 && (v[0] == '"' || v[0] == '\'') && v[n-1] == v[0] {
		q := v[0]
		v = v[1 : n-1]
		if q == '"' {
			v = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(v)
		}
	}
	v = strings.ReplaceAll(v, `\n`, " ")
	return strings.Join(strings.Fields(v), " ")
}

// blockMark is the first token of a yaml block scalar: > or | with the
// optional chomp and indent marks.
var blockMark = regexp.MustCompile(`^[>|][+-]?[0-9]?[+-]?$`)
