package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// transcriptLine is the subset of a Claude Code JSONL entry we care about.
// RequestID and Message.ID together form the dedup key: Claude Code writes one
// line per content block, repeating both across every block of one reply
// (including subagent/sidechain lines, which carry the parent conversation's
// sessionId, so no special-casing is needed there).
type transcriptLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	RequestID string `json:"requestId"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input       int64 `json:"input_tokens"`
			Output      int64 `json:"output_tokens"`
			CacheCreate int64 `json:"cache_creation_input_tokens"`
			CacheRead   int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParseTranscript reads one JSONL transcript, emitting a Record per line that
// carries token usage. Malformed lines are skipped rather than failing the read.
func ParseTranscript(r io.Reader, project string) ([]Record, error) {
	var out []Record
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024) // transcripts can have long lines
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var tl transcriptLine
		if err := json.Unmarshal([]byte(line), &tl); err != nil {
			continue
		}
		if tl.Message.Usage == nil {
			continue
		}
		u := tl.Message.Usage
		ts, _ := time.Parse(time.RFC3339, tl.Timestamp)
		out = append(out, Record{
			Timestamp:   ts,
			Model:       tl.Message.Model,
			Input:       u.Input,
			Output:      u.Output,
			CacheCreate: u.CacheCreate,
			CacheRead:   u.CacheRead,
			Project:     project,
			SessionID:   tl.SessionID,
			Cwd:         tl.Cwd,
			MessageID:   tl.Message.ID,
			RequestID:   tl.RequestID,
		})
	}
	return out, sc.Err()
}

// ScanProjects walks Claude Code's projects dir and parses every .jsonl file,
// using each project subdir name as the project label. a link at the root is
// followed first, since a walk does not descend into one and the projects dir
// of a clalt account is a link to the shared one. a root that is not there is no
// usage yet. a root that is there and cannot be read, or a link that leads
// nowhere, is an error: the numbers would be missing, not zero. what cannot be
// read below the root is skipped.
func ScanProjects(root string) ([]Record, error) {
	real, there, err := resolveRoot(root)
	if err != nil || !there {
		return nil, err
	}
	var all []Record
	err = filepath.WalkDir(real, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == real {
				return err
			}
			return nil // skip unreadable entries
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		project := filepath.Base(filepath.Dir(path))
		recs, _ := ParseTranscript(f, project)
		all = append(all, recs...)
		return nil
	})
	return all, err
}

// resolveRoot follows a link at the root of the projects dir. there is false for
// a root that is not there.
func resolveRoot(root string) (real string, there bool, err error) {
	fi, err := os.Lstat(root)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", false, nil
	case err != nil:
		return "", false, err
	case fi.Mode()&os.ModeSymlink == 0:
		return root, true, nil
	}
	real, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", false, fmt.Errorf("%s is a link that leads nowhere: %w", root, err)
	}
	return real, true, nil
}
