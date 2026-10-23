package claudecli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	signedInJSON  = `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","email":"robin@example.com","orgId":"org-123","orgName":"Example Org","subscriptionType":"max","configDirectory":"@CONFIG_DIR@","projectsDirectory":"@CONFIG_DIR@/projects","analyticsDisabled":false,"addedInTheFuture":{"a":[1,2]}}`
	signedOutJSON = `{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty","configDirectory":"@CONFIG_DIR@","projectsDirectory":"@CONFIG_DIR@/projects","analyticsDisabled":false}`
)

func TestStatusSignedIn(t *testing.T) {
	f := newFake(t)
	setAuthEnv(t)
	f.say("status", 0, signedInJSON)
	dir := t.TempDir()

	st, err := Status(context.Background(), dir)

	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := AuthStatus{
		LoggedIn: true, AuthMethod: "claude.ai", APIProvider: "firstParty",
		Email: "robin@example.com", OrgID: "org-123", OrgName: "Example Org",
		SubscriptionType: "max", ConfigDirectory: dir, ProjectsDirectory: dir + "/projects",
	}
	if st != want {
		t.Errorf("status = %+v, want %+v", st, want)
	}
	if ok, _, text := st.SignedIn(dir); !ok {
		t.Errorf("SignedIn = false (%s)", text)
	}
	if got := f.argv(); !reflect.DeepEqual(got, []string{"auth", "status", "--json"}) {
		t.Errorf("claude was called with %v", got)
	}
	f.assertCleanEnv(dir)
}

func TestStatusSignedOutIsAnAnswerNotAnError(t *testing.T) {
	f := newFake(t)
	f.say("status", 1, signedOutJSON)
	dir := t.TempDir()

	st, err := Status(context.Background(), dir)

	if err != nil {
		t.Fatalf("exit 1 with json should not be an error: %v", err)
	}
	if st.LoggedIn || st.AuthMethod != "none" || st.Email != "" {
		t.Errorf("status = %+v", st)
	}
	if ok, why, text := st.SignedIn(dir); ok || why != ReasonNotSignedIn || text != "not signed in" {
		t.Errorf("SignedIn = %v, %v, %q", ok, why, text)
	}
}

func TestStatusFailuresCarryTheFirstLineOfStderr(t *testing.T) {
	cases := []struct {
		name string
		code int
		out  string
	}{
		{"garbage on success", 0, "this is not json"},
		{"garbage on exit 1", 1, "<html>"},
		{"empty output", 0, ""},
		{"not an object", 0, "[]"},
		{"wrong type", 0, `{"loggedIn":"yes"}`},
		{"other exit code with json", 2, signedOutJSON},
		{"other exit code without json", 137, ""},
	}
	f := newFake(t)
	f.complain("status", "boom: first line\nsecond line")
	for _, c := range cases {
		f.say("status", c.code, c.out)

		_, err := Status(context.Background(), t.TempDir())

		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), "boom: first line") || strings.Contains(err.Error(), "second line") {
			t.Errorf("%s: error = %q, want the first stderr line and only that", c.name, err)
		}
	}
}

func TestStatusWithoutClaudeIsAnError(t *testing.T) {
	skipOnWindows(t)
	isolateHome(t, t.TempDir())
	t.Setenv("CLALT_CLAUDE_BIN", filepath.Join(t.TempDir(), "no-such-claude"))
	if _, err := Status(context.Background(), t.TempDir()); err == nil {
		t.Error("expected an error with no claude binary")
	}
}

func TestSignedIn(t *testing.T) {
	dir := t.TempDir()
	me := "k@example.com"
	cases := []struct {
		name   string
		status AuthStatus
		ok     bool
		why    Reason
		text   string // substring
	}{
		{"claude.ai in this folder", AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", Email: me, ConfigDirectory: dir}, true, ReasonNone, ""},
		{"first party provider named", AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", APIProvider: "firstParty", Email: me, ConfigDirectory: dir}, true, ReasonNone, ""},
		{"signed out", AuthStatus{AuthMethod: "none", ConfigDirectory: dir}, false, ReasonNotSignedIn, "not signed in"},
		{"api key helper", AuthStatus{LoggedIn: true, AuthMethod: "api_key_helper", Email: me, ConfigDirectory: dir}, false, ReasonOtherAuthMethod, "api_key_helper"},
		{"no auth method", AuthStatus{LoggedIn: true, Email: me, ConfigDirectory: dir}, false, ReasonOtherAuthMethod, "unknown"},
		{"other provider", AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", APIProvider: "bedrock", Email: me, ConfigDirectory: dir}, false, ReasonOtherProvider, "bedrock"},
		// an expired or incomplete credential reports loggedIn true and no email
		{"no email", AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", ConfigDirectory: dir}, false, ReasonNoEmail, "email"},
		{"no folder reported", AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", Email: me}, false, ReasonNoFolder, "folder"},
		{"other folder", AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", Email: me, ConfigDirectory: "/somewhere/else"}, false, ReasonOtherFolder, "/somewhere/else"},
	}
	for _, c := range cases {
		ok, why, text := c.status.SignedIn(dir)
		if ok != c.ok || why != c.why || !strings.Contains(text, c.text) || (ok && text != "") {
			t.Errorf("%s: SignedIn = %v, %v, %q, want %v, %v and %q", c.name, ok, why, text, c.ok, c.why, c.text)
		}
	}
}

func TestSignedInComparesFoldersAfterResolvingSymlinks(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// claude reports the path it resolved, clalt may have used the link
	st := AuthStatus{LoggedIn: true, AuthMethod: "claude.ai", Email: "k@example.com", ConfigDirectory: real}
	for _, asked := range []string{real, link} {
		if ok, _, text := st.SignedIn(asked); !ok {
			t.Errorf("SignedIn(%s) = false (%s)", asked, text)
		}
	}
}

// the same cases as TestSignedIn, but read from what the fake claude prints.
func TestStatusThroughTheFakeFlagsOtherIdentities(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()
	cases := []struct {
		name, out string
		why       Reason
		text      string
	}{
		{"api key helper", `{"loggedIn":true,"authMethod":"api_key_helper","email":"k@example.com","configDirectory":"@CONFIG_DIR@"}`, ReasonOtherAuthMethod, "api_key_helper"},
		{"other folder", `{"loggedIn":true,"authMethod":"claude.ai","email":"k@example.com","configDirectory":"/somewhere/else"}`, ReasonOtherFolder, "/somewhere/else"},
		{"third party provider", `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"bedrock","email":"k@example.com","configDirectory":"@CONFIG_DIR@"}`, ReasonOtherProvider, "bedrock"},
		{"email is null", `{"loggedIn":true,"authMethod":"claude.ai","email":null,"configDirectory":"@CONFIG_DIR@"}`, ReasonNoEmail, "email"},
	}
	for _, c := range cases {
		f.say("status", 0, c.out)

		st, err := Status(context.Background(), dir)

		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if ok, why, text := st.SignedIn(dir); ok || why != c.why || !strings.Contains(text, c.text) {
			t.Errorf("%s: SignedIn = %v, %v, %q, want false, %v and a reason naming %q", c.name, ok, why, text, c.why, c.text)
		}
	}
}
