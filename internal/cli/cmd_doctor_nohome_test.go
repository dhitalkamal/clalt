package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// with no way to tell where the claude home is, doctor must not read what
// happens to be in the working directory instead.
func TestDoctorDoesNotLookInTheWorkingDirectoryWhenTheClaudeHomeIsUnknown(t *testing.T) {
	newEnv(t)
	cwd := t.TempDir()
	for name, body := range map[string]string{
		"settings.json":     `{"apiKeyHelper":"x"}`,
		".credentials.json": `{}`,
	} {
		if err := os.WriteFile(filepath.Join(cwd, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(cwd)
	t.Setenv("HOME", "")

	c := settingsCheck()
	if c.Status != statusWarn || !strings.Contains(c.Detail, "HOME") || strings.Contains(c.Detail, "apiKeyHelper") {
		t.Errorf("settings check = %+v, want a warning that the home is unknown", c)
	}
	c = credentialsCheck()
	if c.Status != statusWarn || !strings.Contains(c.Detail, "HOME") || strings.Contains(c.Detail, ".credentials.json is there") {
		t.Errorf("credentials check = %+v, want a warning that the home is unknown", c)
	}
}
