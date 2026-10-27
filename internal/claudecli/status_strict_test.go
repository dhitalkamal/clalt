package claudecli

import (
	"context"
	"strings"
	"testing"
)

func TestStatusRequiresTheKeysItDecidesOn(t *testing.T) {
	f := newFake(t)
	cases := []struct {
		name string
		code int
		out  string
		key  string
	}{
		{"no loggedIn", 0, `{"authMethod":"claude.ai","email":"k@example.com","configDirectory":"@CONFIG_DIR@"}`, "loggedIn"},
		{"no authMethod", 0, `{"loggedIn":true,"email":"k@example.com","configDirectory":"@CONFIG_DIR@"}`, "authMethod"},
		{"no configDirectory", 0, `{"loggedIn":true,"authMethod":"claude.ai","email":"k@example.com"}`, "configDirectory"},
		{"signed out without authMethod", 1, `{"loggedIn":false,"configDirectory":"@CONFIG_DIR@"}`, "authMethod"},
		{"signed out without configDirectory", 1, `{"loggedIn":false,"authMethod":"none"}`, "configDirectory"},
		{"null is not an object", 0, `null`, "json"},
	}
	for _, c := range cases {
		f.say("status", c.code, c.out)

		_, err := Status(context.Background(), t.TempDir())

		if err == nil || !strings.Contains(err.Error(), c.key) {
			t.Errorf("%s: err = %v, want one that names %s", c.name, err, c.key)
		}
	}
}

func TestStatusExitCodeMustAgreeWithLoggedIn(t *testing.T) {
	f := newFake(t)
	for _, c := range []struct {
		name string
		code int
		out  string
		ok   bool
	}{
		{"signed in, exit 0", 0, signedInJSON, true},
		{"signed out, exit 1", 1, signedOutJSON, true},
		{"signed out but exit 0", 0, signedOutJSON, false},
		{"signed in but exit 1", 1, signedInJSON, false},
	} {
		f.say("status", c.code, c.out)

		st, err := Status(context.Background(), t.TempDir())

		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
		if err == nil && st.ExitCode != c.code {
			t.Errorf("%s: ExitCode = %d, want %d", c.name, st.ExitCode, c.code)
		}
	}
}
