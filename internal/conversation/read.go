// Package conversation lists the conversations claude keeps in the shared
// projects folder, across accounts: which session they are, where they ran,
// what they are called, when they were last used and on which model.
package conversation

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Conversation is one transcript in the projects folder.
type Conversation struct {
	ID         string // claude's session id, the file name without .jsonl
	Project    string // the folder under projects: the cwd as claude spells it
	Path       string
	Cwd        string
	Title      string
	LastActive time.Time
	Model      string
	Size       int64
	Entrypoint string // cli, sdk-cli, as claude writes it
}

// maxLine is the longest transcript line that is read, as in internal/usage.
const maxLine = 16 * 1024 * 1024

// titleRunes is how much of a first prompt is kept.
const titleRunes = 100

// maxCwds bounds how many distinct folders one transcript can offer.
const maxCwds = 16

// record is the part of a transcript line that is looked at. the entry format
// is internal to claude and changes, so every field is optional.
type record struct {
	Type             string          `json:"type"`
	Timestamp        string          `json:"timestamp"`
	Cwd              string          `json:"cwd"`
	Entrypoint       string          `json:"entrypoint"`
	IsMeta           bool            `json:"isMeta"`
	IsSidechain      bool            `json:"isSidechain"`
	IsCompactSummary bool            `json:"isCompactSummary"`
	Origin           json.RawMessage `json:"origin"`
	CustomTitle      string          `json:"customTitle"`
	AITitle          string          `json:"aiTitle"`
	Summary          string          `json:"summary"`
	Message          struct {
		Model string `json:"model"`
	} `json:"message"`
}

// content is read only for user lines, and only until a prompt is found, since
// it holds the bulk of a line.
type content struct {
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// describe reads one transcript. ok is false for a file that holds no user or
// assistant record, which is not a conversation.
func describe(path, project string) (c Conversation, ok bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return c, false, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return c, false, err
	}
	c = Conversation{
		ID:      strings.TrimSuffix(fi.Name(), ".jsonl"),
		Project: project, Path: path, Size: fi.Size(),
	}

	var custom, generated, summary, prompt string
	var cwds []string
	seenCwd := map[string]bool{}
	err = readLines(f, func(line []byte) {
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			var mismatch *json.UnmarshalTypeError
			if !errors.As(err, &mismatch) {
				return
			}
		}
		if t, err := time.Parse(time.RFC3339, r.Timestamp); err == nil && t.After(c.LastActive) {
			c.LastActive = t
		}
		if r.Cwd != "" && !seenCwd[r.Cwd] && len(cwds) < maxCwds {
			seenCwd[r.Cwd] = true
			cwds = append(cwds, r.Cwd)
		}
		if r.Entrypoint != "" && c.Entrypoint == "" {
			c.Entrypoint = r.Entrypoint
		}
		switch r.Type {
		case "custom-title":
			custom = r.CustomTitle
		case "ai-title":
			generated = r.AITitle
		case "summary":
			summary = r.Summary
		case "assistant":
			ok = true
			if m := r.Message.Model; m != "" && m != "<synthetic>" {
				c.Model = m
			}
		case "user":
			ok = true
			if prompt == "" && isHumanPrompt(r) {
				prompt = firstPrompt(line)
			}
		}
	})
	if err != nil {
		return c, false, err
	}
	if !ok {
		return c, false, nil
	}
	for _, t := range []string{custom, generated, summary} {
		if t = tidy(t); t != "" {
			c.Title = t
			break
		}
	}
	if c.Title == "" {
		c.Title = prompt
	}
	c.Cwd = pickCwd(cwds, project)
	if c.LastActive.IsZero() {
		c.LastActive = fi.ModTime()
	}
	return c, true, nil
}

// isHumanPrompt is false for the user records that are not something the
// person typed: meta messages, subagent traffic, compaction summaries, and
// prompts that came from somewhere else (task notifications, peers).
func isHumanPrompt(r record) bool {
	if r.IsMeta || r.IsSidechain || r.IsCompactSummary {
		return false
	}
	if len(r.Origin) > 0 && r.Origin[0] == '{' {
		var o struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(r.Origin, &o) == nil && o.Kind != "" && o.Kind != "human" {
			return false
		}
	}
	return true
}

// firstPrompt is the text of a user line, cut short, or "" when the line holds
// no text of a person: tool results, images, and command or system wrappers
// (text that starts with "<").
func firstPrompt(line []byte) string {
	var c content
	if json.Unmarshal(line, &c) != nil {
		return ""
	}
	raw := c.Message.Content
	var text string
	switch {
	case len(raw) > 0 && raw[0] == '"':
		_ = json.Unmarshal(raw, &text)
	case len(raw) > 0 && raw[0] == '[':
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &blocks) != nil {
			return ""
		}
		for _, b := range blocks {
			if b.Type == "text" {
				text = b.Text
				break
			}
		}
	}
	if strings.HasPrefix(strings.TrimSpace(text), "<") {
		return ""
	}
	return cut(tidy(text))
}

// tidy puts text on one line with single spaces.
func tidy(s string) string { return strings.Join(strings.Fields(s), " ") }

// cut keeps at most titleRunes characters, ending at a word when it can, and
// says so with three dots.
func cut(s string) string {
	if utf8.RuneCountInString(s) <= titleRunes {
		return s
	}
	r := []rune(s)[:titleRunes]
	head := string(r)
	if i := strings.LastIndex(head, " "); i > titleRunes/2 {
		head = head[:i]
	}
	return strings.TrimRight(head, " ") + "..."
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// encode is how claude turns a working folder into the name of its project
// folder: every character that is not a letter or digit becomes a dash.
func encode(cwd string) string { return nonAlnum.ReplaceAllString(cwd, "-") }

// pickCwd chooses the folder claude looks the session up by: the one whose
// encoding is the project folder, else the first one seen.
func pickCwd(cwds []string, project string) string {
	for _, c := range cwds {
		if encode(c) == project {
			return c
		}
	}
	if len(cwds) > 0 {
		return cwds[0]
	}
	return ""
}

// readLines calls fn for every line of r. a line longer than maxLine is
// skipped, not fatal, and a last line without a newline is still given.
func readLines(r io.Reader, fn func(line []byte)) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var line []byte
	skipping := false
	for {
		chunk, err := br.ReadSlice('\n')
		if len(chunk) > 0 && !skipping {
			line = append(line, chunk...)
			if len(line) > maxLine {
				line, skipping = nil, true
			}
		}
		switch {
		case err == bufio.ErrBufferFull:
			continue
		case err != nil && err != io.EOF:
			return err
		}
		if !skipping && len(line) > 0 {
			fn(line)
		}
		line, skipping = nil, false
		if err == io.EOF {
			return nil
		}
	}
}
