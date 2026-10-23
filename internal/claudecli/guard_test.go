package claudecli

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// a test that forgot to install a fake must not find the real claude: TestMain
// points CLALT_CLAUDE_BIN at a program that says so and exits 99. this test
// installs no fake on purpose.
func TestATestWithoutAFakeCannotReachTheRealClaude(t *testing.T) {
	bin := os.Getenv("CLALT_CLAUDE_BIN")
	if bin == "" {
		t.Fatal("CLALT_CLAUDE_BIN is not set: a test that forgets its fake would run whatever claude is on PATH")
	}
	out, err := exec.Command(bin, "auth", "status").CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 99 || !strings.Contains(string(out), "a test ran claude without installing a fake") {
		t.Errorf("CLALT_CLAUDE_BIN is %q, which is not the guard: %v %s", bin, err, out)
	}
}
