package reviews

import "testing"

func TestParseFindingsPlainJSON(t *testing.T) {
	f, sum, ok := ParseFindings(`{"summary":"fine","findings":[{"severity":"HIGH","file":"a.go","line":"12","title":"nil","detail":"d"}]}`)
	if !ok || sum != "fine" || len(f) != 1 || f[0].Severity != "high" || f[0].Line != 12 {
		t.Fatalf("%+v %q %v", f, sum, ok)
	}
}

func TestParseFindingsFencedAndChatty(t *testing.T) {
	text := "Here is my review.\n```json\n{\"summary\":\"s\",\"findings\":[{\"severity\":\"nit\",\"file\":\"b.go\",\"line\":4,\"title\":\"t\",\"detail\":\"d\"}]}\n```\nbye"
	f, sum, ok := ParseFindings(text)
	if !ok || sum != "s" || len(f) != 1 || f[0].Severity != "low" {
		t.Fatalf("%+v %q %v (unknown severity words fold to low)", f, sum, ok)
	}
}

func TestParseFindingsNoJSONKeepsTextAsSummary(t *testing.T) {
	f, sum, ok := ParseFindings("looks good to me")
	if ok || len(f) != 0 || sum != "looks good to me" {
		t.Fatalf("%+v %q %v", f, sum, ok)
	}
}

func TestParseFindingsDropsEmptyAndCaps(t *testing.T) {
	f, _, ok := ParseFindings(`{"summary":"s","findings":[{"severity":"low"},{"severity":"medium","title":"x","file":"../../etc/passwd","line":-3}]}`)
	if !ok || len(f) != 1 || f[0].Line != 0 {
		t.Fatalf("%+v (an entry without title is dropped, a negative line is 0)", f)
	}
}

func TestParseClaudeEnvelope(t *testing.T) {
	text, sid := ParseEnvelope(`{"type":"result","result":"hello","session_id":"sess-1"}`)
	if text != "hello" || sid != "sess-1" {
		t.Fatalf("%q %q", text, sid)
	}
	text, _ = ParseEnvelope("raw words")
	if text != "raw words" {
		t.Fatal(text)
	}
}
