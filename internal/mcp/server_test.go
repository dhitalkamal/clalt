package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// reply is a decoded JSON-RPC response line, with the parts the tests read.
type reply struct {
	ID     json.RawMessage `json:"id"`
	Result map[string]any  `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// talk feeds lines to a server built on b and returns the reply lines.
func talk(t *testing.T, b Backend, lines ...string) []reply {
	t.Helper()
	var out bytes.Buffer
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	if err := Serve(context.Background(), in, &out, b); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var got []reply
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var r reply
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("stdout line is not json: %q", l)
		}
		got = append(got, r)
	}
	return got
}

func TestInitializeEchoesASupportedVersion(t *testing.T) {
	got := talk(t, Backend{}, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"x","version":"1"}}}`)
	if len(got) != 1 || got[0].Error != nil {
		t.Fatalf("reply = %+v", got)
	}
	r := got[0].Result
	if r["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v", r["protocolVersion"])
	}
	caps := r["capabilities"].(map[string]any)
	if _, ok := caps["tools"]; !ok || len(caps) != 1 {
		t.Errorf("capabilities = %v, want only tools", caps)
	}
	if r["serverInfo"].(map[string]any)["name"] != "clalt" {
		t.Errorf("serverInfo = %v", r["serverInfo"])
	}
}

func TestInitializeAnswersAnUnknownVersionWithOurLatest(t *testing.T) {
	got := talk(t, Backend{}, `{"jsonrpc":"2.0","id":"a","method":"initialize","params":{"protocolVersion":"1999-01-01"}}`)
	if got[0].Result["protocolVersion"] != latestVersion {
		t.Errorf("protocolVersion = %v, want %s", got[0].Result["protocolVersion"], latestVersion)
	}
	if string(got[0].ID) != `"a"` {
		t.Errorf("id = %s, want the string id back", got[0].ID)
	}
}

func TestNotificationsGetNoReply(t *testing.T) {
	got := talk(t, Backend{},
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/whatever"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if len(got) != 1 || string(got[0].ID) != "2" || got[0].Error != nil {
		t.Errorf("replies = %+v, want only the ping answer", got)
	}
}

func TestUnknownMethodIsMethodNotFound(t *testing.T) {
	got := talk(t, Backend{}, `{"jsonrpc":"2.0","id":7,"method":"resources/list"}`)
	if got[0].Error == nil || got[0].Error.Code != -32601 {
		t.Errorf("reply = %+v, want -32601", got[0])
	}
}

func TestBadJSONIsAParseErrorAndTheLoopGoesOn(t *testing.T) {
	got := talk(t, Backend{}, `{not json`, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if len(got) != 2 || got[0].Error == nil || got[0].Error.Code != -32700 || string(got[0].ID) != "null" {
		t.Fatalf("replies = %+v", got)
	}
	if got[1].Error != nil {
		t.Errorf("the ping after the bad line failed: %+v", got[1])
	}
}

func TestBatchAndNonObjectAreInvalidRequests(t *testing.T) {
	got := talk(t, Backend{}, `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, `42`, `{"jsonrpc":"1.0","id":3,"method":"ping"}`)
	for i, r := range got {
		if r.Error == nil || r.Error.Code != -32600 {
			t.Errorf("reply %d = %+v, want -32600", i, r)
		}
	}
	if len(got) != 3 {
		t.Errorf("got %d replies, want 3", len(got))
	}
}

func TestEveryReplyIsOneLine(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	if err := Serve(context.Background(), in, &out, Backend{}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 1 || !strings.HasSuffix(out.String(), "\n") {
		t.Errorf("reply is not exactly one line: %q", out.String())
	}
}

func TestDiscoverAnswersTheModernProbe(t *testing.T) {
	got := talk(t, Backend{}, `{"jsonrpc":"2.0","id":"d","method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`)
	if got[0].Error != nil {
		t.Fatalf("reply = %+v", got[0])
	}
	if got[0].Result["resultType"] != "complete" || got[0].Result["supportedVersions"] == nil {
		t.Errorf("result = %v", got[0].Result)
	}
}

func TestAnOversizedLineIsRefusedNotFatal(t *testing.T) {
	big := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"x":"` + strings.Repeat("a", maxLine+10) + `"}}`
	got := talk(t, Backend{}, big, `{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if len(got) != 2 || got[0].Error == nil || got[1].Error != nil {
		t.Errorf("replies = %+v", got)
	}
}
