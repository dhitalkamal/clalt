package officed

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// plistValue reads one plist value whose start element was already consumed.
// dicts become map[string]any, arrays []any, strings string, anything else nil.
func plistValue(t *testing.T, dec *xml.Decoder, start xml.StartElement) any {
	t.Helper()
	switch start.Name.Local {
	case "dict":
		m := map[string]any{}
		key := ""
		for {
			tok, err := dec.Token()
			if err != nil {
				t.Fatalf("read dict: %v", err)
			}
			switch e := tok.(type) {
			case xml.EndElement:
				return m
			case xml.StartElement:
				if e.Name.Local == "key" {
					var k string
					if err := dec.DecodeElement(&k, &e); err != nil {
						t.Fatalf("read key: %v", err)
					}
					key = k
					continue
				}
				m[key] = plistValue(t, dec, e)
			}
		}
	case "array":
		var arr []any
		for {
			tok, err := dec.Token()
			if err != nil {
				t.Fatalf("read array: %v", err)
			}
			switch e := tok.(type) {
			case xml.EndElement:
				return arr
			case xml.StartElement:
				arr = append(arr, plistValue(t, dec, e))
			}
		}
	case "string":
		var s string
		if err := dec.DecodeElement(&s, &start); err != nil {
			t.Fatalf("read string: %v", err)
		}
		return s
	default:
		if err := dec.Skip(); err != nil {
			t.Fatalf("skip %s: %v", start.Name.Local, err)
		}
		return nil
	}
}

// parsePlist returns the top-level dict, failing the test on malformed xml.
func parsePlist(t *testing.T, data []byte) map[string]any {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			t.Fatalf("no top-level dict in:\n%s", data)
		}
		if err != nil {
			t.Fatalf("plist does not parse: %v\n%s", err, data)
		}
		if s, ok := tok.(xml.StartElement); ok && s.Name.Local == "dict" {
			return plistValue(t, dec, s).(map[string]any)
		}
	}
}

// envPath digs EnvironmentVariables -> PATH out of a plist.
func envPath(t *testing.T, data []byte) (string, bool) {
	t.Helper()
	env, ok := parsePlist(t, data)["EnvironmentVariables"].(map[string]any)
	if !ok {
		return "", false
	}
	p, ok := env["PATH"].(string)
	return p, ok
}

func TestPlistCarriesPathFromTheFieldInAnEnvironmentVariablesDict(t *testing.T) {
	a, _ := newAutostart(t)
	a.PATH = "/usr/local/bin:/usr/bin:/bin"
	got, ok := envPath(t, a.Plist())
	if !ok {
		t.Fatalf("no EnvironmentVariables -> PATH in:\n%s", a.Plist())
	}
	if got != a.PATH {
		t.Errorf("PATH = %q, want %q", got, a.PATH)
	}
}

func TestPlistDropsDuplicateAndEmptyPathEntriesKeepingFirstSeenOrder(t *testing.T) {
	a, _ := newAutostart(t)
	a.PATH = "/a::/b:/a:/c:"
	got, ok := envPath(t, a.Plist())
	if !ok {
		t.Fatalf("no PATH in:\n%s", a.Plist())
	}
	if want := "/a:/b:/c"; got != want {
		t.Errorf("PATH = %q, want %q", got, want)
	}
}

func TestPlistXMLEscapesThePathAndStillParses(t *testing.T) {
	a, _ := newAutostart(t)
	a.PATH = "/opt/a&b:/opt/<x>/bin"
	got, ok := envPath(t, a.Plist())
	if !ok {
		t.Fatalf("no PATH in:\n%s", a.Plist())
	}
	if got != a.PATH {
		t.Errorf("PATH = %q, want %q", got, a.PATH)
	}
}

func TestPlistOmitsEnvironmentVariablesWhenPathIsEmpty(t *testing.T) {
	for _, in := range []string{"", ":::"} {
		a, _ := newAutostart(t)
		a.PATH = in
		m := parsePlist(t, a.Plist())
		if _, has := m["EnvironmentVariables"]; has {
			t.Errorf("PATH %q: EnvironmentVariables present:\n%s", in, a.Plist())
		}
		if m["Label"] != Label {
			t.Errorf("PATH %q: Label = %v, want %q", in, m["Label"], Label)
		}
	}
}

func TestPlistWithPathPassesPlutilLint(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("plutil is darwin only")
	}
	bin, err := exec.LookPath("plutil")
	if err != nil {
		t.Skip("plutil not found")
	}
	a, _ := newAutostart(t)
	a.PATH = "/opt/a&b:/usr/bin:/usr/bin:/bin"
	file := filepath.Join(t.TempDir(), "job.plist")
	if err := os.WriteFile(file, a.Plist(), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "-lint", file).CombinedOutput(); err != nil {
		t.Errorf("plutil -lint: %v\n%s", err, out)
	}
	if p, ok := envPath(t, a.Plist()); !ok || p == "" {
		t.Errorf("no PATH in:\n%s", a.Plist())
	}
}
