package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorSignInChecks(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(e *testEnv)
		checks map[string][2]string // check -> status, detail
		exit   int
	}{
		{
			name: "signed out",
			setup: func(e *testEnv) {
				e.signIn("work", "w@example.com")
				e.claude.signOut(e.folder("work"))
			},
			checks: map[string][2]string{"signedIn": {"warn", "clalt login work"}},
		},
		{
			name: "signed in as somebody else",
			setup: func(e *testEnv) {
				e.signIn("work", "w@example.com")
				e.claude.plantLogin(e.folder("work"), "other@example.com")
			},
			checks: map[string][2]string{"signedIn": {"ok", ""}, "identity": {"fail", "w@example.com"}},
			exit:   1,
		},
		{
			name: "no recorded email",
			setup: func(e *testEnv) {
				if _, err := e.app.createAccount("work"); err != nil {
					t.Fatal(err)
				}
				e.claude.plantLogin(accountDir(t, "work"), "w@example.com")
			},
			checks: map[string][2]string{"identity": {"warn", "clalt login work"}},
		},
		{
			name: "an api key helper in the folder's place",
			setup: func(e *testEnv) {
				e.signIn("work", "w@example.com")
				e.claude.set("API_KEY_HELPER", "1")
			},
			checks: map[string][2]string{"signedIn": {"warn", "api_key_helper"}},
		},
		{
			name: "a status check that breaks",
			setup: func(e *testEnv) {
				e.signIn("work", "w@example.com")
				e.claude.set("STATUS_BROKEN", "1")
			},
			checks: map[string][2]string{"signedIn": {"warn", "keychain is locked"}},
		},
		{
			name: "claude reports a login without an email",
			setup: func(e *testEnv) {
				e.signIn("work", "w@example.com")
				e.claude.plantLogin(e.folder("work"), "")
			},
			checks: map[string][2]string{"signedIn": {"warn", "without an email"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.setup(e)

			code, rep := doctor(t, e)

			for check, want := range c.checks {
				rep.want(t, "work", check, want[0], want[1])
			}
			if code != c.exit {
				t.Errorf("exit %d, want %d", code, c.exit)
			}
		})
	}
}

func TestDoctorWarnsAboutALoginThatIsDueSoonOrOverdue(t *testing.T) {
	cases := []struct {
		name   string
		after  time.Duration
		status string
		detail string
	}{
		{"plenty of time", 10 * 24 * time.Hour, "ok", ""},
		{"just outside the window", 26*24*time.Hour + time.Hour, "ok", ""},
		{"inside the window", 28 * 24 * time.Hour, "warn", "sign in again by"},
		{"due today", 30*24*time.Hour - time.Hour, "warn", "sign in again by"},
		{"overdue", 31 * 24 * time.Hour, "warn", "was due"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.at(theClock)
			e.signIn("work", "w@example.com")
			e.at(theClock.Add(c.after))

			code, rep := doctor(t, e)

			rep.want(t, "work", "loginDue", c.status, c.detail)
			if code != 0 {
				t.Errorf("a login that is due is a warning, exit %d", code)
			}
		})
	}
}

func TestDoctorNamesAuthVariablesButNeverTheirValues(t *testing.T) {
	e := newEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-secret-value")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "tok-secret-value")

	_, rep := doctor(t, e)

	rep.want(t, "shell", "authEnv", "warn", "ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN")
	if strings.Contains(e.out.String(), "secret-value") {
		t.Errorf("a value leaked:\n%s", e.out.String())
	}
}

func TestDoctorFindsAnApiKeyHelperAndAuthVariablesInTheSharedSettings(t *testing.T) {
	e := newEnv(t)
	settings := `{"apiKeyHelper":"/usr/local/bin/get-key --secret=hunter2","env":{"ANTHROPIC_API_KEY":"sk-secret-value","EDITOR":"vim"},"model":"opus"}`
	if err := os.MkdirAll(filepath.Join(e.home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.home, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	rep.want(t, "settings", "authSettings", "warn", "apiKeyHelper")
	rep.want(t, "settings", "authSettings", "warn", "ANTHROPIC_API_KEY")
	out := e.out.String()
	for _, leak := range []string{"hunter2", "secret-value", "get-key", "EDITOR", "vim"} {
		if strings.Contains(out, leak) {
			t.Errorf("%q leaked into the report:\n%s", leak, out)
		}
	}
}

func TestDoctorChecksTheLocalSettingsToo(t *testing.T) {
	e := newEnv(t)
	if err := os.MkdirAll(filepath.Join(e.home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.home, ".claude", "settings.local.json"), []byte(`{"apiKeyHelper":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	rep.want(t, "settings", "authSettings", "warn", "settings.local.json")
}

func TestDoctorSaysWhenSettingsCannotBeRead(t *testing.T) {
	e := newEnv(t)
	if err := os.MkdirAll(filepath.Join(e.home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.home, ".claude", "settings.json"), []byte(`{"apiKeyHelper": "tok-secret`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, rep := doctor(t, e)

	rep.want(t, "settings", "authSettings", "warn", "settings.json")
	if strings.Contains(e.out.String(), "tok-secret") {
		t.Error("part of the file leaked into the report")
	}
}
