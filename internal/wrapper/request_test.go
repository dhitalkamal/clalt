package wrapper

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

const handle = "3fa9c1d2e4b5a607"

var at = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func TestWriteAndClaimARequest(t *testing.T) {
	dir := t.TempDir()
	want := Request{To: "personal", RequestedAt: at}

	if err := WriteRequest(dir, handle, want); err != nil {
		t.Fatal(err)
	}
	if !HasRequest(dir, handle) {
		t.Error("HasRequest is false for a request that was written")
	}
	got, ok, err := ClaimRequest(dir, handle)

	if err != nil || !ok || got.To != "personal" || !got.RequestedAt.Equal(at) {
		t.Errorf("ClaimRequest = %+v, %v, %v", got, ok, err)
	}
	left, _ := os.ReadDir(dir)
	if len(left) != 0 || HasRequest(dir, handle) {
		t.Errorf("a claimed request left %v behind", left)
	}
}

func TestClaimingWithNoRequestIsNotAnError(t *testing.T) {
	if _, ok, err := ClaimRequest(t.TempDir(), handle); ok || err != nil {
		t.Errorf("ok = %v, err = %v", ok, err)
	}
	if _, ok, err := ClaimRequest(filepath.Join(t.TempDir(), "no-such-dir"), handle); ok || err != nil {
		t.Errorf("without the folder: ok = %v, err = %v", ok, err)
	}
}

func TestANewerRequestReplacesAnUnclaimedOne(t *testing.T) {
	dir := t.TempDir()
	for _, to := range []string{"a", "b"} {
		if err := WriteRequest(dir, handle, Request{To: to, RequestedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	if got, _, _ := ClaimRequest(dir, handle); got.To != "b" {
		t.Errorf("claimed %q, want the newer one", got.To)
	}
}

// the file is not named like a record, so a registry that lists *.json and
// removes what it cannot read never meets it
func TestTheRequestFileIsNotJSONAndIsPrivate(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRequest(dir, handle, Request{To: "a", RequestedAt: at}); err != nil {
		t.Fatal(err)
	}
	path, _ := RequestPath(dir, handle)
	if filepath.Dir(path) != dir || !strings.HasSuffix(path, handle+".move") || strings.HasSuffix(path, ".json") {
		t.Errorf("path = %s", path)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, %v", fi.Mode(), err)
		}
	}
}

func TestOnlyAHandleBecomesAPath(t *testing.T) {
	for _, h := range []string{"", "../x", "3fa9c1d2e4b5a60", "3fa9c1d2e4b5a6070", "3FA9C1D2E4B5A607", "3fa9c1d2e4b5a60g", "a/b/c/d/e/f/g/h"} {
		if ValidHandle(h) {
			t.Errorf("%q is taken for a handle", h)
		}
		if _, err := RequestPath(t.TempDir(), h); err == nil {
			t.Errorf("RequestPath accepted %q", h)
		}
		if err := WriteRequest(t.TempDir(), h, Request{To: "a"}); err == nil {
			t.Errorf("WriteRequest accepted %q", h)
		}
	}
	if !ValidHandle(handle) {
		t.Error("a real handle is refused")
	}
}

func TestAMalformedRequestIsAnErrorAndIsGone(t *testing.T) {
	for name, body := range map[string]string{
		"not json": "move to work please", "no target": `{"version":1}`, "other version": `{"version":2,"to":"a"}`, "empty": "",
	} {
		dir := t.TempDir()
		path, _ := RequestPath(dir, handle)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}

		_, ok, err := ClaimRequest(dir, handle)

		if ok || err == nil {
			t.Errorf("%s: ok = %v, err = %v", name, ok, err)
		}
		if left, _ := os.ReadDir(dir); len(left) != 0 {
			t.Errorf("%s: left %v behind, it would be tried again every time", name, left)
		}
	}
}

func TestARequestWithoutATargetIsNotWritten(t *testing.T) {
	if err := WriteRequest(t.TempDir(), handle, Request{}); err == nil {
		t.Error("WriteRequest accepted a request to nowhere")
	}
}

func TestRemoveRequestsClearsBothNames(t *testing.T) {
	dir := t.TempDir()
	path, _ := RequestPath(dir, handle)
	for _, p := range []string{path, path + ".claimed"} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	RemoveRequests(dir, handle)
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("left %v", left)
	}
	RemoveRequests(dir, handle) // nothing there is fine
}

func TestMatchFindsASessionByHandlePidOrIdPrefix(t *testing.T) {
	recs := []Record{
		{Handle: "aaaaaaaaaaaaaaaa", WrapperPID: 4321, ClaudeSessionID: "7c5dcf5d-1111-4222-8333-444455556666"},
		{Handle: "bbbbbbbbbbbbbbbb", WrapperPID: 99, ClaudeSessionID: "7c5e0000-1111-4222-8333-444455556666"},
		{Handle: "cccccccccccccccc", WrapperPID: 7, ClaudeSessionID: "01234567-1111-4222-8333-444455556666"},
	}
	cases := []struct {
		ref, want, err string
	}{
		{"bbbbbbbbbbbbbbbb", "bbbbbbbbbbbbbbbb", ""},
		{"4321", "aaaaaaaaaaaaaaaa", ""},
		{"7c5d", "aaaaaaaaaaaaaaaa", ""},
		{"7C5DCF5D-1111", "aaaaaaaaaaaaaaaa", ""},
		{"0123", "cccccccccccccccc", ""},
		{"7c5", "", "too short"},
		{"7c5d0", "", "no running session"},
		{"7c5", "", "too short"},
		{"", "", "which session"},
		{"123456", "", "no running session"},
	}
	for _, c := range cases {
		got, err := Match(recs, c.ref)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("Match(%q) error = %v, want %q", c.ref, err, c.err)
			}
			continue
		}
		if err != nil || got.Handle != c.want {
			t.Errorf("Match(%q) = %q, %v, want %q", c.ref, got.Handle, err, c.want)
		}
	}
}

func TestMatchRefusesAnAmbiguousPrefixAndNamesTheChoices(t *testing.T) {
	recs := []Record{
		{Handle: "aaaaaaaaaaaaaaaa", ClaudeSessionID: "7c5dcf5d-1"},
		{Handle: "bbbbbbbbbbbbbbbb", ClaudeSessionID: "7c5ecf5e-2"},
	}
	_, err := Match(recs, "7c5d")
	if err != nil {
		t.Fatalf("7c5d is only the first: %v", err)
	}
	_, err = Match(recs, "7c5")
	if err == nil {
		t.Fatal("a three character prefix was accepted")
	}
	recs[1].ClaudeSessionID = "7c5dcf5d-2"
	_, err = Match(recs, "7c5d")
	if err == nil || !strings.Contains(err.Error(), "aaaaaaaaaaaaaaaa") || !strings.Contains(err.Error(), "bbbbbbbbbbbbbbbb") {
		t.Errorf("error = %v, want both handles named", err)
	}
}

// a handle that happens to be all digits is a handle, not a pid
func TestMatchPrefersAHandleToAPid(t *testing.T) {
	recs := []Record{
		{Handle: "1234567890123456", WrapperPID: 5},
		{Handle: "bbbbbbbbbbbbbbbb", WrapperPID: 1234567890123456},
	}
	got, err := Match(recs, "1234567890123456")
	if err != nil || got.Handle != "1234567890123456" {
		t.Errorf("Match = %+v, %v", got, err)
	}
}

func TestResumeArgsCarriesWhatAResumeDoesNotRestore(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nothing", nil, []string{"--resume", "S"}},
		{"the first prompt is not carried", []string{"fix the bug"}, []string{"--resume", "S"}},
		{"flags a resume restores are left out", []string{"--model", "opus", "--permission-mode", "plan", "--agent", "x", "--effort", "high", "-n", "name"},
			[]string{"--resume", "S"}},
		{"settings and friends", []string{"--settings", "s.json", "--fallback-model", "sonnet", "--plugin-dir", "p"},
			[]string{"--resume", "S", "--settings", "s.json", "--fallback-model", "sonnet", "--plugin-dir", "p"}},
		{"the equals form", []string{"--settings=s.json", "--plugin-dir=p"}, []string{"--resume", "S", "--settings=s.json", "--plugin-dir=p"}},
		{"variadic flags take every value up to the next flag", []string{"--add-dir", "../a", "../b", "--model", "opus", "--mcp-config", "m1.json", "m2.json"},
			[]string{"--resume", "S", "--add-dir", "../a", "../b", "--mcp-config", "m1.json", "m2.json"}},
		{"strict goes with mcp-config", []string{"--strict-mcp-config", "--mcp-config", "m.json"}, []string{"--resume", "S", "--strict-mcp-config", "--mcp-config", "m.json"}},
		{"bypass flags", []string{"--dangerously-skip-permissions", "--allow-dangerously-skip-permissions"},
			[]string{"--resume", "S", "--dangerously-skip-permissions", "--allow-dangerously-skip-permissions"}},
		{"bypass by mode", []string{"--permission-mode", "bypassPermissions"}, []string{"--resume", "S", "--permission-mode", "bypassPermissions"}},
		{"bypass by mode with equals", []string{"--permission-mode=bypassPermissions"}, []string{"--resume", "S", "--permission-mode=bypassPermissions"}},
		{"another mode is not carried", []string{"--permission-mode=acceptEdits"}, []string{"--resume", "S"}},
		{"resume and continue are replaced", []string{"--resume", "old", "-c", "--continue", "-r", "x", "--session-id", "u", "--fork-session"}, []string{"--resume", "S"}},
		{"after the double dash is the prompt", []string{"--", "--settings", "x"}, []string{"--resume", "S"}},
		{"a flag with no value is dropped", []string{"--settings"}, []string{"--resume", "S"}},
		{"a value is not mistaken for a flag", []string{"--model", "--settings"}, []string{"--resume", "S"}},
	}
	for _, c := range cases {
		if got := ResumeArgs(c.in, "S"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: ResumeArgs(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestAResumeDoesNotLoseAFlagThatFollowsAValuelessPermissionMode(t *testing.T) {
	got := ResumeArgs([]string{"--permission-mode", "--add-dir", "../a"}, "S")
	if want := []string{"--resume", "S", "--add-dir", "../a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ResumeArgs = %v, want %v", got, want)
	}
}

func TestAFailedMoveLeavesAResultThatIsTakenOnce(t *testing.T) {
	dir := t.TempDir()
	if _, ok := TakeResult(dir, handle); ok {
		t.Fatal("a result from nowhere")
	}
	if err := WriteResult(dir, handle, Result{To: "personal", Error: "not signed in", At: at}); err != nil {
		t.Fatal(err)
	}
	got, ok := TakeResult(dir, handle)
	if !ok || got.To != "personal" || got.Error != "not signed in" {
		t.Errorf("TakeResult = %+v, %v", got, ok)
	}
	if _, ok := TakeResult(dir, handle); ok {
		t.Error("a result was taken twice")
	}
}
