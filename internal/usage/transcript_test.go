package usage

import (
	"strings"
	"testing"
)

const fixture = `
{"type":"user","timestamp":"2026-08-24T09:59:00.000Z","message":{"role":"user"}}
{"type":"assistant","timestamp":"2026-08-24T10:00:00.000Z","sessionId":"s1","cwd":"/Users/x/proj","message":{"model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":50,"cache_read_input_tokens":30}}}
{"type":"assistant","timestamp":"2026-08-24T10:05:00.000Z","sessionId":"s1","message":{"model":"claude-sonnet-5","usage":{"input_tokens":10,"output_tokens":20}}}
{"type":"summary","summary":"noise line, no usage"}
not-json-at-all
`

func TestParseTranscript(t *testing.T) {
	recs, err := ParseTranscript(strings.NewReader(fixture), "proj")
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	// only the two assistant lines with usage should produce records
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2: %+v", len(recs), recs)
	}
	r0 := recs[0]
	if r0.Model != "claude-opus-4-8" {
		t.Errorf("model = %q", r0.Model)
	}
	if r0.Input != 100 || r0.Output != 200 || r0.CacheCreate != 50 || r0.CacheRead != 30 {
		t.Errorf("tokens wrong: %+v", r0)
	}
	if r0.TotalTokens() != 380 {
		t.Errorf("TotalTokens = %d, want 380", r0.TotalTokens())
	}
	if r0.Project != "proj" || r0.SessionID != "s1" {
		t.Errorf("meta wrong: %+v", r0)
	}
	if r0.Timestamp.IsZero() {
		t.Error("timestamp not parsed")
	}
}
