package profile

import (
	"path/filepath"
	"testing"
)

func TestPlanNeverLinksCredentialFilesInsideASplitFolder(t *testing.T) {
	root, shared := sharedFixture(t)
	splitShared(t, shared)
	for _, n := range []string{"skills/.credentials.json", "plugins/.claude.json"} {
		writeT(t, filepath.Join(shared, filepath.FromSlash(n)), "secret")
	}
	acct := filepath.Join(root, "acct")
	// a link for one of them, from before the rule, has to go
	mustSymlink(t, filepath.Join(shared, "plugins", ".claude.json"), filepath.Join(acct, "plugins", ".claude.json"))

	got := stepMap(mustPlan(t, acct, shared))

	if st, ok := got["skills/.credentials.json"]; ok {
		t.Errorf("planned %+v for a credentials file", st)
	}
	if st := got["plugins/.claude.json"]; st.Action != Remove {
		t.Errorf("plugins/.claude.json = %+v, want the old link removed", st)
	}
}
