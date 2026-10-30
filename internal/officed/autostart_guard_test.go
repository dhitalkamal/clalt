package officed

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// topKeys lists the top-level plist keys, sorted.
func topKeys(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func TestPlistKeepsItsExistingKeysAndValues(t *testing.T) {
	a, _ := newAutostart(t)
	a.PATH = "/usr/local/bin:/usr/bin"
	raw := string(a.Plist())
	m := parsePlist(t, a.Plist())

	if m["Label"] != "com.dhitalkamal.clalt.office" {
		t.Errorf("Label = %v", m["Label"])
	}
	wantArgs := []any{a.Exe, "office", "serve"}
	if !reflect.DeepEqual(m["ProgramArguments"], wantArgs) {
		t.Errorf("ProgramArguments = %v, want %v", m["ProgramArguments"], wantArgs)
	}
	// the parser skips booleans, so check them in the raw text
	for _, want := range []string{"<key>RunAtLoad</key>\n\t<true/>", "<key>KeepAlive</key>\n\t<false/>"} {
		if !strings.Contains(raw, want) {
			t.Errorf("plist lacks %q in:\n%s", want, raw)
		}
	}
	if m["StandardOutPath"] != a.LogPath {
		t.Errorf("StandardOutPath = %v, want %q", m["StandardOutPath"], a.LogPath)
	}
	if m["StandardErrorPath"] != a.LogPath {
		t.Errorf("StandardErrorPath = %v, want %q", m["StandardErrorPath"], a.LogPath)
	}
}

func TestPlistTopLevelKeysAreExactlyTheKnownSet(t *testing.T) {
	base := []string{"KeepAlive", "Label", "ProgramArguments", "RunAtLoad", "StandardErrorPath", "StandardOutPath"}
	withEnv := append(append([]string{}, base...), "EnvironmentVariables")
	sort.Strings(withEnv)

	a, _ := newAutostart(t)
	a.PATH = "/usr/bin"
	if got := topKeys(parsePlist(t, a.Plist())); !reflect.DeepEqual(got, withEnv) {
		t.Errorf("keys with PATH = %v, want %v", got, withEnv)
	}

	a.PATH = ""
	if got := topKeys(parsePlist(t, a.Plist())); !reflect.DeepEqual(got, base) {
		t.Errorf("keys without PATH = %v, want %v", got, base)
	}
}
