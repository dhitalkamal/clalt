package summary

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// maxLine is the longest transcript line read, as in internal/usage.
const maxLine = 16 << 20

// field caps, in characters.
const (
	maxFailText = 120
	maxFileName = 200
	maxToolName = 60
)

type evKind int

const (
	evTool evKind = iota
	evSubagent
	evFailure
)

// event is one thing that happened inside the wanted time window.
type event struct {
	at      time.Time
	session string
	kind    evKind
	name    string // tool name, or the subagent type
	text    string // first line of a failed result
	file    string
}

// scan is what the files gave: usage records, events in the window, and the
// timestamps and working folder of each session.
type scan struct {
	records []usage.Record
	events  []event
	stamps  map[string][]int64
	cwd     map[string]string
	dir     map[string]string // session -> project folder name of its file
	seen    map[string]bool   // tool_use ids already counted
	skipped int
}

func newScan() *scan {
	return &scan{stamps: map[string][]int64{}, cwd: map[string]string{}, dir: map[string]string{}, seen: map[string]bool{}}
}

type rawLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	RequestID string `json:"requestId"`
	Message   struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			Input       int64 `json:"input_tokens"`
			Output      int64 `json:"output_tokens"`
			CacheCreate int64 `json:"cache_creation_input_tokens"`
			CacheRead   int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type rawBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

// window is the span events and stamps are kept for. a zero window keeps none.
type window struct{ from, to time.Time }

func (w window) has(t time.Time) bool { return !w.from.IsZero() && !t.Before(w.from) && t.Before(w.to) }

// readFile adds one transcript to the scan. project is the folder name the
// file sits under, kept on the records until a working folder is known.
func (s *scan) readFile(r io.Reader, project string, w window) {
	br := bufio.NewReaderSize(r, 64*1024)
	tools := map[string]string{} // tool_use id -> tool name, for results in this file
	for {
		raw, err := readLine(br)
		if len(raw) > 0 {
			s.line(raw, project, w, tools)
		}
		if err != nil {
			return
		}
	}
}

// readLine returns one line, or skips it whole when over maxLine.
func readLine(br *bufio.Reader) ([]byte, error) {
	var buf []byte
	over := false
	for {
		chunk, err := br.ReadSlice('\n')
		if !over {
			buf = append(buf, chunk...)
			if len(buf) > maxLine {
				over, buf = true, nil
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return bytes.TrimSpace(buf), err
	}
}

func (s *scan) line(raw []byte, project string, w window, tools map[string]string) {
	var l rawLine
	if err := json.Unmarshal(raw, &l); err != nil {
		s.skipped++
		return
	}
	ts, err := time.Parse(time.RFC3339Nano, l.Timestamp)
	if err != nil {
		return
	}
	if u := l.Message.Usage; u != nil {
		s.records = append(s.records, usage.Record{
			Timestamp: ts, Model: l.Message.Model, Input: u.Input, Output: u.Output,
			CacheCreate: u.CacheCreate, CacheRead: u.CacheRead, Project: project,
			SessionID: l.SessionID, Cwd: l.Cwd, MessageID: l.Message.ID, RequestID: l.RequestID,
		})
	}
	if l.SessionID != "" && s.dir[l.SessionID] == "" {
		s.dir[l.SessionID] = project
	}
	if l.SessionID != "" && l.Cwd != "" && s.cwd[l.SessionID] == "" {
		s.cwd[l.SessionID] = l.Cwd
	}
	if w.from.IsZero() {
		return
	}
	if !w.has(ts) || l.SessionID == "" {
		// results are paired by id, so the calls of other days still count as known
		if bytes.Contains(raw, []byte(`"tool_use"`)) {
			s.blocks(l, ts, w, tools, false)
		}
		return
	}
	s.stamps[l.SessionID] = append(s.stamps[l.SessionID], ts.Unix())
	if bytes.Contains(raw, []byte(`"tool_use"`)) || bytes.Contains(raw, []byte(`"tool_result"`)) {
		s.blocks(l, ts, w, tools, true)
	}
}

// blocks reads the tool calls and failed results of a line. keep is false for
// a line outside the window: only the ids are learned.
func (s *scan) blocks(l rawLine, ts time.Time, w window, tools map[string]string, keep bool) {
	var blocks []rawBlock
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return
	}
	cwd := l.Cwd
	for _, b := range blocks {
		switch b.Type {
		case "tool_use":
			tools[b.ID] = b.Name
			if !keep || (b.ID != "" && s.seen[b.ID]) {
				continue
			}
			s.seen[b.ID] = true
			s.toolEvent(l.SessionID, ts, b, cwd)
		case "tool_result":
			if !keep || !b.IsError {
				continue
			}
			name := tools[b.ToolUseID]
			if name == "" {
				name = "unknown"
			}
			s.events = append(s.events, event{at: ts, session: l.SessionID, kind: evFailure,
				name: clean(name, maxToolName), text: firstLine(contentText(b.Content))})
		}
	}
}

func (s *scan) toolEvent(session string, ts time.Time, b rawBlock, cwd string) {
	var in map[string]json.RawMessage
	_ = json.Unmarshal(b.Input, &in)
	str := func(k string) string {
		var v string
		_ = json.Unmarshal(in[k], &v)
		return v
	}
	if b.Name == "Agent" || b.Name == "Task" {
		typ := clean(str("subagent_type"), maxToolName)
		if typ == "" {
			typ = "unknown"
		}
		s.events = append(s.events, event{at: ts, session: session, kind: evSubagent, name: typ})
		return
	}
	e := event{at: ts, session: session, kind: evTool, name: clean(b.Name, maxToolName)}
	for _, k := range []string{"file_path", "notebook_path", "filePath"} {
		if f := str(k); f != "" {
			e.file = clean(relTo(f, cwd), maxFileName)
			break
		}
	}
	s.events = append(s.events, e)
}

// contentText reads a result content that is text or a list of text blocks.
func contentText(raw json.RawMessage) string {
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return str
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		parts = append(parts, b.Text)
	}
	return strings.Join(parts, "\n")
}

// firstLine is the first non empty line, cleaned and cut.
func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := clean(ln, maxFailText); t != "" {
			return t
		}
	}
	return ""
}
