package office

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dhitalkamal/clalt/internal/feed"
	"github.com/dhitalkamal/clalt/internal/office/activity"
	"github.com/dhitalkamal/clalt/internal/registry"
)

var (
	nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)
	// sessionFile is the id of a claude session as it names a file. it comes
	// from a record on disk, so only these characters are let through.
	sessionFile = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
)

// encodeCwd is how claude turns a working folder into the name of its project
// folder, as in internal/conversation.
func encodeCwd(cwd string) string { return nonAlnum.ReplaceAllString(cwd, "-") }

// Transcript finds the transcript of a session by its handle: the claude
// session id comes from the registry (clalt run) or from the handle itself
// (plain claude), the project folder from the working folder, with a scan of
// the projects folders when the name does not match. a subagent run is the
// file in the session's subagents folder, else the sidechain lines of the main
// file.
func (r Real) Transcript(handle, agent string) (activity.Source, error) {
	id, cwd, err := r.claudeSession(handle)
	if err != nil {
		return activity.Source{}, err
	}
	if !sessionFile.MatchString(id) {
		return activity.Source{}, &NotFoundError{Reason: "the session has no transcript yet"}
	}
	main := r.findTranscript(id, cwd)
	if main == "" {
		return activity.Source{}, &NotFoundError{Reason: "no transcript of this session was found"}
	}
	if agent == "" {
		return activity.Source{Path: main}, nil
	}
	sub := filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents", "agent-"+agent+".jsonl")
	if fi, err := os.Stat(sub); err == nil && fi.Mode().IsRegular() {
		return activity.Source{Path: sub, Agent: agent}, nil
	}
	return activity.Source{Path: main, Agent: agent, Legacy: true}, nil
}

// claudeSession is the claude session id and working folder of a handle.
func (r Real) claudeSession(handle string) (id, cwd string, err error) {
	if st, ok := r.Feed.Session(handle); ok {
		cwd = st.Cwd
	}
	if feed.IsExt(handle) {
		return strings.TrimPrefix(handle, "ext-"), cwd, nil
	}
	if !registry.ValidHandle(handle) {
		return "", "", &NotFoundError{Reason: "not a session handle"}
	}
	rec, ok, err := r.Registry.Get(handle)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", &NotFoundError{Reason: "no such session"}
	}
	if cwd == "" {
		cwd = rec.Cwd
	}
	return rec.ClaudeSessionID, cwd, nil
}

// findTranscript looks in the project folder of the working folder first, then
// in every project folder. empty when there is no such file.
func (r Real) findTranscript(id, cwd string) string {
	name := id + ".jsonl"
	for _, root := range r.Projects {
		if cwd != "" {
			if p := filepath.Join(root, encodeCwd(cwd), name); isFile(p) {
				return p
			}
		}
	}
	for _, root := range r.Projects {
		dirs, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, d := range dirs {
			if p := filepath.Join(root, d.Name(), name); isFile(p) {
				return p
			}
		}
	}
	return ""
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
