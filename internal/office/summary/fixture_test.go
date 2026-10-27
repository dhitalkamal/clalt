package summary

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// all text in these fixtures is invented.

// mst is a fixed zone seven hours behind utc, so a local day is not a utc day.
var mst = time.FixedZone("MST", -7*3600)

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }

// asst is an assistant line with usage and optional content blocks.
func asst(ts, sid, cwd, msg string, in, out int64, blocks ...string) string {
	content := "[" + strings.Join(blocks, ",") + "]"
	return fmt.Sprintf(`{"type":"assistant","timestamp":%s,"sessionId":%s,"cwd":%s,"requestId":"req-%s","message":{"id":%s,"model":"claude-sonnet-4-5","content":%s,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`,
		jsonStr(ts), jsonStr(sid), jsonStr(cwd), msg, jsonStr(msg), content, in, out)
}

func toolUse(id, name, input string) string {
	return fmt.Sprintf(`{"type":"tool_use","id":%q,"name":%q,"input":%s}`, id, name, input)
}

// userLine carries tool results.
func userLine(ts, sid, cwd string, blocks ...string) string {
	return fmt.Sprintf(`{"type":"user","timestamp":%s,"sessionId":%s,"cwd":%s,"message":{"content":[%s]}}`, jsonStr(ts), jsonStr(sid), jsonStr(cwd), strings.Join(blocks, ","))
}

func toolResult(id string, isErr bool, text string) string {
	return fmt.Sprintf(`{"type":"tool_result","tool_use_id":%q,"is_error":%t,"content":%s}`, id, isErr, jsonStr(text))
}

// write puts lines in <root>/<proj>/<name>.jsonl with the given mtime.
func write(t *testing.T, root, proj, name string, mtime time.Time, lines ...string) string {
	t.Helper()
	dir := filepath.Join(root, proj)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name+".jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return p
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// bySession attributes records by a fixed session to account table.
func bySession(m map[string]string) func([]usage.Record) (map[string][]usage.Record, error) {
	return func(recs []usage.Record) (map[string][]usage.Record, error) {
		out := map[string][]usage.Record{}
		for _, r := range recs {
			a := m[r.SessionID]
			if a == "" {
				a = usage.Unattributed
			}
			out[a] = append(out[a], r)
		}
		return out, nil
	}
}

func accounts() ([]Account, error) {
	return []Account{{ID: "personal", Label: "PERSONAL"}, {ID: "work", Label: "WORK"}}, nil
}

// newService is a service over root with the fixed clock now (utc) in mst.
func newService(root, now string, m map[string]string) *Service {
	return New(Config{
		Roots: []string{root}, Accounts: accounts, Attribute: bySession(m),
		Now: func() time.Time { return at(now) }, Loc: mst,
	})
}
