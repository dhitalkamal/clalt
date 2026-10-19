package usage

import "testing"

// TestDedupWithinFile proves repeated (message.id, requestId) pairs collapse to
// one record, keeping the first occurrence (Claude Code writes one JSONL line
// per content block, repeating the same usage on every block of one reply).
func TestDedupWithinFile(t *testing.T) {
	recs := []Record{
		{MessageID: "msg_1", RequestID: "req_1", Input: 10},
		{MessageID: "msg_1", RequestID: "req_1", Input: 10}, // duplicate content block
		{MessageID: "msg_1", RequestID: "req_1", Input: 10}, // duplicate content block
		{MessageID: "msg_2", RequestID: "req_1", Input: 5},
	}
	got := Dedup(recs)
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2: %+v", len(got), got)
	}
	if got[0].MessageID != "msg_1" || got[1].MessageID != "msg_2" {
		t.Errorf("dedup kept wrong records: %+v", got)
	}
}

// TestDedupAcrossFiles proves the same (message.id, requestId) pair appearing
// in two different files (e.g. a resumed session rewriting earlier history)
// still collapses to one record when deduping a combined scan.
func TestDedupAcrossFiles(t *testing.T) {
	fileA := []Record{{MessageID: "msg_1", RequestID: "req_1", Input: 10}}
	fileB := []Record{{MessageID: "msg_1", RequestID: "req_1", Input: 10}, {MessageID: "msg_3", RequestID: "req_3", Input: 1}}
	got := Dedup(append(append([]Record{}, fileA...), fileB...))
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2: %+v", len(got), got)
	}
}

// TestDedupKeepsRecordsWithoutMessageID proves records with no message.id are
// never dropped, since there is no id to dedup them by.
func TestDedupKeepsRecordsWithoutMessageID(t *testing.T) {
	recs := []Record{
		{MessageID: "", RequestID: "", Input: 1},
		{MessageID: "", RequestID: "", Input: 1},
	}
	got := Dedup(recs)
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 (no message.id to dedup by): %+v", len(got), got)
	}
}

// TestDedupSameMessageIDDifferentRequestID proves the dedup key is the PAIR
// (message.id, requestId), not message.id alone.
func TestDedupSameMessageIDDifferentRequestID(t *testing.T) {
	recs := []Record{
		{MessageID: "msg_1", RequestID: "req_1", Input: 1},
		{MessageID: "msg_1", RequestID: "req_2", Input: 1},
	}
	got := Dedup(recs)
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 (different requestId): %+v", len(got), got)
	}
}
