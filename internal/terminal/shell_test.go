package terminal

import "testing"

func TestShellLine(t *testing.T) {
	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"clalt", "run", "work"}, "clalt run work"},
		{[]string{"clalt", "run", "my work"}, "clalt run 'my work'"},
		{[]string{"a", "it's"}, `a 'it'\''s'`},
		{[]string{"a", ""}, "a ''"},
		{[]string{"a", "$HOME", "x;y", "a*b", "~"}, "a '$HOME' 'x;y' 'a*b' '~'"},
		{[]string{"--resume=abc/def.1_2-3:4@5+6,7%8"}, "--resume=abc/def.1_2-3:4@5+6,7%8"},
		{[]string{"a\nb"}, "'a\nb'"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := ShellLine(c.argv); got != c.want {
			t.Errorf("ShellLine(%q) = %q, want %q", c.argv, got, c.want)
		}
	}
}
