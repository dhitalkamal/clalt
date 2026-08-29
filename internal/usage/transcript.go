package usage

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// transcriptLine is the subset of a Claude Code JSONL entry we care about.
type transcriptLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Message   struct {
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
		})
	}
	return out, sc.Err()
}

// ScanProjects walks Claude Code's projects dir and parses every .jsonl file,
// using each project subdir name as the project label.
func ScanProjects(root string) ([]Record, error) {
	var all []Record
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
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
	if os.IsNotExist(err) {
		return nil, nil
	}
	return all, err
}
