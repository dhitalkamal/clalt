package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// seedRepeatedReply writes a transcript in which one reply appears as two
// lines with the same message id and request id, the way Claude Code writes
// one line per content block.
func seedRepeatedReply(t *testing.T, dir string) {
	t.Helper()
	projDir := filepath.Join(dir, "projects", "proj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"assistant","timestamp":"2026-08-24T10:00:00Z","sessionId":"s1","requestId":"req_1","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":100,"output_tokens":200}}}` + "\n"
	if err := os.WriteFile(filepath.Join(projDir, "s.jsonl"), []byte(line+line), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestRecordsByAccountDedupsRepeatedReplyLines proves the report path scans
// through usage.ScanRoots, so a reply written as several transcript lines
// counts once. clalt usage and the dashboard data both build on
// recordsByAccount.
func TestRecordsByAccountDedupsRepeatedReplyLines(t *testing.T) {
	app, _, dir := newTestApp(t)
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	seedRepeatedReply(t, dir)

	byAccount, err := app.recordsByAccount()
	if err != nil {
		t.Fatalf("recordsByAccount: %v", err)
	}
	total := 0
	for _, recs := range byAccount {
		total += len(recs)
	}
	if total != 1 {
		t.Errorf("records = %d, want 1 (two lines of one reply deduped)", total)
	}
}

// TestUsageCountsRepeatedReplyOnceUnderActiveAccount proves deduping does not
// change attribution: with no switch history the one reply still folds into
// the active account, and no separate unattributed report appears.
func TestUsageCountsRepeatedReplyOnceUnderActiveAccount(t *testing.T) {
	app, _, dir := newTestApp(t)
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	seedRepeatedReply(t, dir)
	_ = app.Live.Write(liveCred("tok"))
	_ = app.cmdAdd([]string{"main"}) // becomes active, no switch events

	reports, err := app.buildReports(false, "")
	if err != nil {
		t.Fatalf("buildReports: %v", err)
	}
	if len(reports) != 1 || reports[0].Name != "main" {
		t.Fatalf("reports = %+v, want exactly the active account main", reports)
	}
	if reports[0].Tokens != 300 {
		t.Errorf("main tokens = %d, want 300 (one reply, not two)", reports[0].Tokens)
	}
}
