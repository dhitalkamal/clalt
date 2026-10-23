package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// every refusal happens before any terminal program is called.
func TestLaunchRefusals(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *testEnv, dir string)
		args  func(dir string) []string
		code  int
		want  []string
	}{
		{
			name:  "no account",
			setup: func(*testEnv, string) {},
			args:  func(string) []string { return []string{"launch"} },
			code:  2,
			want:  []string{"which account", "usage: clalt launch <name>"},
		},
		{
			name:  "unknown flag",
			setup: func(*testEnv, string) {},
			args:  func(string) []string { return []string{"launch", "work", "--bogus"} },
			code:  2,
			want:  []string{"unknown flag --bogus", "usage: clalt launch <name>"},
		},
		{
			name:  "a second account",
			setup: func(*testEnv, string) {},
			args:  func(string) []string { return []string{"launch", "work", "other"} },
			code:  2,
			want:  []string{"unexpected argument other"},
		},
		{
			name:  "cwd without a value",
			setup: func(*testEnv, string) {},
			args:  func(string) []string { return []string{"launch", "work", "--cwd"} },
			code:  2,
			want:  []string{"--cwd needs a folder"},
		},
		{
			name:  "an account that does not exist",
			setup: func(*testEnv, string) {},
			args:  func(string) []string { return []string{"launch", "nobody"} },
			code:  1,
			want:  []string{`no account called "nobody"`},
		},
		{
			name: "an account that was removed",
			setup: func(e *testEnv, _ string) {
				if err := e.app.Store.Hide(e.account("work").Name); err != nil {
					t.Fatal(err)
				}
			},
			args: func(string) []string { return []string{"launch", "work"} },
			code: 1,
			want: []string{`"work" was removed`, "clalt login work"},
		},
		{
			name:  "an account that signed out",
			setup: func(e *testEnv, _ string) { e.claude.signOut(e.folder("work")) },
			args:  func(string) []string { return []string{"launch", "work"} },
			code:  1,
			want:  []string{"work is not ready to run: ", "not signed in", "clalt login work"},
		},
		{
			name:  "a folder signed in as somebody else",
			setup: func(e *testEnv, _ string) { e.claude.plantLogin(e.folder("work"), "other@example.com") },
			args:  func(string) []string { return []string{"launch", "work"} },
			code:  1,
			want:  []string{"work is not ready to run: ", "clalt login work"},
		},
		{
			name:  "a folder that is not there",
			setup: func(*testEnv, string) {},
			args:  func(dir string) []string { return []string{"launch", "work", "--cwd", filepath.Join(dir, "nope")} },
			code:  1,
			want:  []string{"--cwd", "nope"},
		},
		{
			name: "a file instead of a folder",
			setup: func(_ *testEnv, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "f"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			args: func(dir string) []string { return []string{"launch", "work", "--cwd", filepath.Join(dir, "f")} },
			code: 1,
			want: []string{"not a folder"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, f, dir := launchEnv(t)
			c.setup(e, dir)

			code := e.run(c.args(dir)...)

			if code != c.code {
				t.Errorf("exit %d, want %d (stderr %q)", code, c.code, e.errOut.String())
			}
			for _, w := range c.want {
				if !strings.Contains(e.errOut.String(), w) {
					t.Errorf("stderr %q lacks %q", e.errOut.String(), w)
				}
			}
			if len(f.run.Calls()) != 0 || e.out.Len() != 0 {
				t.Errorf("calls %v, stdout %q", f.run.Calls(), e.out.String())
			}
		})
	}
}

func TestLaunchWithNoStateCreatesNone(t *testing.T) {
	e := newEnv(t)
	e.useFakeDeps()

	code := e.run("launch", "nobody")

	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".config", "clalt")); err == nil {
		t.Error("launch created the clalt folder")
	}
}
