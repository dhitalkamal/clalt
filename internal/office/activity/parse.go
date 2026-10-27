package activity

import (
	"encoding/json"
	"strings"
	"time"
)

// the entry kinds of the contract.
const (
	KindPrompt        = "prompt"
	KindReply         = "reply"
	KindTool          = "tool"
	KindResult        = "result"
	KindSubagentStart = "subagent-start"
	KindSubagentEnd   = "subagent-end"
	KindNote          = "note"
)

// limits of one field and one line.
const (
	maxText      = 20000
	maxFiles     = 20
	maxFileRunes = 500
	maxBlocks    = 100 // blocks read from one line, so one line cannot fill a reply
)

// rawLine is the part of a transcript line that is looked at. the format is
// claude's own and changes, so everything loose is kept raw.
type rawLine struct {
	Type          string          `json:"type"`
	Subtype       string          `json:"subtype"`
	Level         string          `json:"level"`
	Timestamp     string          `json:"timestamp"`
	IsSidechain   bool            `json:"isSidechain"`
	IsMeta        bool            `json:"isMeta"`
	AgentID       string          `json:"agentId"`
	Content       json.RawMessage `json:"content"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
	Message       struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// block is one element of a message's content list.
type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

// resultInfo is what a tool result says about its tool call.
type resultInfo struct {
	idx   int // index of the result entry in the reply
	at    time.Time
	ok    bool
	agent string
	async bool
}

// piece is one entry of a line before pairing: a tool call carries its id.
type piece struct {
	Entry
	useID, useName string
	res            *resultInfo
	resID          string
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func newEntry(kind string, t time.Time, text string) Entry {
	return Entry{T: stamp(t), Kind: kind, Text: cleanText(text, maxText), Files: []string{}, OK: true}
}

// textOf reads a content that is a string or a list of text blocks.
func textOf(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// pieces turns one line into entries in file order. a line that is not
// something to show gives none.
func pieces(l rawLine, t time.Time) []piece {
	switch l.Type {
	case "system":
		return systemNote(l, t)
	case "user", "assistant":
	default:
		return nil
	}
	if l.IsMeta {
		return nil
	}
	var s string
	if json.Unmarshal(l.Message.Content, &s) == nil {
		if l.Type == "user" && strings.TrimSpace(s) != "" {
			return []piece{{Entry: newEntry(KindPrompt, t, s)}}
		}
		return nil
	}
	var blocks []block
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return nil
	}
	if len(blocks) > maxBlocks {
		blocks = blocks[:maxBlocks]
	}
	var out []piece
	for _, b := range blocks {
		switch {
		case b.Type == "text" && strings.TrimSpace(b.Text) != "":
			kind := KindReply
			if l.Type == "user" {
				kind = KindPrompt
			}
			out = append(out, piece{Entry: newEntry(kind, t, b.Text)})
		case b.Type == "tool_use" && l.Type == "assistant":
			out = append(out, toolPiece(b, t))
		case b.Type == "tool_result" && l.Type == "user":
			out = append(out, resultPiece(b, l, t))
		}
	}
	return out
}

// systemNote shows the system lines that matter to a reader: a compaction and
// errors. hook summaries and the like are noise.
func systemNote(l rawLine, t time.Time) []piece {
	var text string
	_ = json.Unmarshal(l.Content, &text)
	switch {
	case l.Subtype == "compact_boundary":
		if text == "" {
			text = "conversation compacted"
		}
	case l.Level == "error" && text != "":
	default:
		return nil
	}
	return []piece{{Entry: newEntry(KindNote, t, text)}}
}

func isAgentTool(name string) bool { return name == "Agent" || name == "Task" }

// pathKeys are the tool input fields that name a file or folder.
var pathKeys = []string{"file_path", "notebook_path", "path", "filePath"}

// textKeys are looked at in order for the one line that says what a call does.
var textKeys = []string{"command", "file_path", "notebook_path", "pattern", "url", "query", "description", "prompt", "path"}

func toolPiece(b block, t time.Time) piece {
	var in map[string]json.RawMessage
	_ = json.Unmarshal(b.Input, &in)
	str := func(key string) string {
		var s string
		_ = json.Unmarshal(in[key], &s)
		return s
	}
	kind, keys := KindTool, textKeys
	if isAgentTool(b.Name) {
		kind, keys = KindSubagentStart, []string{"description", "subagent_type", "prompt"}
	}
	var text string
	for _, k := range keys {
		if text = str(k); text != "" {
			break
		}
	}
	e := newEntry(kind, t, text)
	e.Tool = cleanText(b.Name, 60)
	var files []string
	for _, k := range pathKeys {
		if s := str(k); s != "" {
			files = append(files, s)
		}
	}
	for _, k := range []string{"paths", "file_paths"} {
		var list []string
		if json.Unmarshal(in[k], &list) == nil {
			files = append(files, list...)
		}
	}
	seen := map[string]bool{}
	for _, f := range files {
		if f = cleanText(f, maxFileRunes); f != "" && !seen[f] && len(e.Files) < maxFiles {
			seen[f] = true
			e.Files = append(e.Files, f)
		}
	}
	return piece{Entry: e, useID: b.ID, useName: b.Name}
}

func resultPiece(b block, l rawLine, t time.Time) piece {
	e := newEntry(KindResult, t, textOf(b.Content))
	e.OK = !b.IsError
	info := &resultInfo{at: t, ok: e.OK}
	var tur struct {
		AgentID  string `json:"agentId"`
		AgentID2 string `json:"agent_id"`
		IsAsync  bool   `json:"isAsync"`
		Status   string `json:"status"`
	}
	if json.Unmarshal(l.ToolUseResult, &tur) == nil {
		info.agent = cleanText(firstOf(tur.AgentID, tur.AgentID2), 100)
		info.async = tur.IsAsync || tur.Status == "async_launched"
	}
	return piece{Entry: e, resID: b.ToolUseID, res: info}
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
