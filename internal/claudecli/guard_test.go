package claudecli

import (
	"os"
	"strings"
	"testing"
)

// a test that forgot to install a fake must not find the real claude: TestMain
// points CLALT_CLAUDE_BIN at a script that says so and fails. this test installs
// no fake on purpose.
func TestATestWithoutAFakeCannotReachTheRealClaude(t *testing.T) {
	skipOnWindows(t)
	bin := os.Getenv("CLALT_CLAUDE_BIN")
	if bin == "" {
		t.Fatal("CLALT_CLAUDE_BIN is not set: a test that forgets its fake would run whatever claude is on PATH")
	}
	raw, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("CLALT_CLAUDE_BIN is %q: %v", bin, err)
	}
	if len(raw) > 1024 || !strings.Contains(string(raw), "a test ran claude without installing a fake") {
		t.Errorf("CLALT_CLAUDE_BIN is %q, which is not the guard script", bin)
	}
}
