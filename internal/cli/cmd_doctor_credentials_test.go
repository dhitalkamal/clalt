package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plantCredentials puts an unreadable-looking login file in the shared home.
func plantCredentials(t *testing.T, e *testEnv) string {
	t.Helper()
	path := filepath.Join(e.home, ".claude", ".credentials.json")
	if err := os.WriteFile(path, []byte(`{"claudeAiOauth":{"accessToken":"tok-secret-value"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCredentialsFileOfASignedInDefaultHomeIsKept(t *testing.T) {
	e := newEnv(t)
	path := plantCredentials(t, e)
	e.claude.plantLogin(filepath.Join(e.home, ".claude"), "me@example.com")

	c := credentialsCheck()

	if c.Status != statusOK || !strings.Contains(c.Detail, path) || !strings.Contains(c.Detail, "must stay") {
		t.Fatalf("status %s, detail %q", c.Status, c.Detail)
	}
	if strings.Contains(c.Detail, "tok-secret-value") {
		t.Error("the file's content leaked")
	}
}

func TestCredentialsFileOfASignedOutDefaultHomeIsALeftover(t *testing.T) {
	e := newEnv(t)
	path := plantCredentials(t, e)

	c := credentialsCheck()

	if c.Status != statusWarn || !strings.Contains(c.Detail, path) || !strings.Contains(c.Detail, "leftover") || !strings.Contains(c.Detail, "can go") {
		t.Fatalf("status %s, detail %q", c.Status, c.Detail)
	}
	if strings.Contains(c.Detail, "claude auth status") {
		t.Errorf("doctor still tells the user to ask claude by hand: %q", c.Detail)
	}
}

func TestCredentialsFileKeepsTheCarefulWarningWhenClaudeCannotAnswer(t *testing.T) {
	e := newEnv(t)
	path := plantCredentials(t, e)
	e.claude.set("STATUS_BROKEN", "1")

	c := credentialsCheck()

	if c.Status != statusWarn || !strings.Contains(c.Detail, path) || !strings.Contains(c.Detail, "claude auth status") {
		t.Fatalf("status %s, detail %q", c.Status, c.Detail)
	}
}

func TestCredentialsFileKeepsTheCarefulWarningForALoginThatIsNotClaudeAi(t *testing.T) {
	e := newEnv(t)
	plantCredentials(t, e)
	e.claude.plantLogin(filepath.Join(e.home, ".claude"), "me@example.com")
	e.claude.set("API_KEY_HELPER", "1")

	c := credentialsCheck()

	if c.Status != statusWarn || !strings.Contains(c.Detail, "claude auth status") {
		t.Fatalf("status %s, detail %q", c.Status, c.Detail)
	}
}
