package reviews

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxFindings = 50
	maxTitle    = 200
	maxDetail   = 2000
	maxFile     = 300
	maxSummary  = 4000
)

// ParseEnvelope reads the output of claude -p --output-format json: the
// reply text and the session id. anything that is not that envelope is taken
// as the reply text itself.
func ParseEnvelope(raw string) (text, sessionID string) {
	var env struct {
		Result    string `json:"result"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &env); err == nil && (env.Result != "" || env.SessionID != "") {
		return env.Result, env.SessionID
	}
	return raw, ""
}

// tidy drops control characters (newlines and tabs stay) and caps the length.
func tidy(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r) {
			return r
		}
		return -1
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

func severity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "critical", "blocker", "error", "major":
		return "high"
	case "medium", "warning", "moderate":
		return "medium"
	}
	return "low"
}

// candidates are the pieces of text that may be the reviewer's JSON object:
// the whole text, a fenced block, then the span from the first brace to the
// last.
func candidates(text string) []string {
	t := strings.TrimSpace(text)
	out := []string{t}
	if i := strings.Index(t, "```"); i >= 0 {
		rest := t[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		if j := strings.Index(rest, "```"); j >= 0 {
			out = append(out, strings.TrimSpace(rest[:j]))
		}
	}
	if a, b := strings.Index(t, "{"), strings.LastIndex(t, "}"); a >= 0 && b > a {
		out = append(out, t[a:b+1])
	}
	return out
}

// ParseFindings reads the findings and summary out of the reviewer's reply.
// the prompt asks for {"summary": "...", "findings": [{severity, file, line,
// title, detail}]} but the reply may be fenced or wrapped in chat. ok is false
// when no such object was found, and then the whole text is the summary.
func ParseFindings(text string) (findings []Finding, summary string, ok bool) {
	for _, c := range candidates(text) {
		var raw struct {
			Summary  string `json:"summary"`
			Findings []struct {
				Severity string `json:"severity"`
				File     string `json:"file"`
				Line     any    `json:"line"`
				Title    string `json:"title"`
				Detail   string `json:"detail"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(c), &raw); err != nil || (raw.Summary == "" && raw.Findings == nil) {
			continue
		}
		findings = []Finding{}
		for _, f := range raw.Findings {
			if strings.TrimSpace(f.Title) == "" || len(findings) >= maxFindings {
				continue
			}
			findings = append(findings, Finding{
				Severity: severity(f.Severity), File: tidy(f.File, maxFile), Line: lineOf(f.Line),
				Title: tidy(f.Title, maxTitle), Detail: tidy(f.Detail, maxDetail),
			})
		}
		return findings, tidy(raw.Summary, maxSummary), true
	}
	return []Finding{}, tidy(text, maxSummary), false
}

// lineOf accepts a number or a numeric string, anything else is 0 (no line).
func lineOf(v any) int {
	var n int
	switch x := v.(type) {
	case float64:
		n = int(x)
	case string:
		n, _ = strconv.Atoi(strings.TrimSpace(x))
	}
	if n < 0 || n > 10_000_000 {
		return 0
	}
	return n
}
