package usage

// Dedup collapses records that share the same (MessageID, RequestID) pair,
// keeping the first occurrence. Claude Code writes one JSONL line per content
// block, repeating the same message usage on every block of one reply, so a
// naive sum over all usage-bearing lines overcounts by roughly the average
// number of content blocks per reply. Records with no MessageID cannot be
// deduped (there is nothing to key on) and are always kept as-is.
//
// Callers are expected to pass records in their natural read order (file walk
// order, then line order within a file) so "first occurrence" is deterministic
// and stable across repeated runs.
func Dedup(records []Record) []Record {
	seen := make(map[dedupKey]struct{}, len(records))
	out := make([]Record, 0, len(records))
	for _, r := range records {
		if r.MessageID == "" {
			out = append(out, r)
			continue
		}
		key := dedupKey{messageID: r.MessageID, requestID: r.RequestID}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out
}

type dedupKey struct {
	messageID string
	requestID string
}
