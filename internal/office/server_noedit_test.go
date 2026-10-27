package office

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// the write routes need no --edit: the guards are what protect them.
func TestWritesWorkWithoutEditFlag(t *testing.T) {
	e := newGenEnv(t, "")
	if rec := e.send("POST", "/api/layout/use", `{"name":"river"}`); rec.Code != 200 || e.store.ActiveName() != "river" {
		t.Fatalf("use: %d %s", rec.Code, rec.Body)
	}
	if rec := e.send("PUT", "/api/layout", string(encode(t, "mine"))); rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if _, err := e.store.Load("hall-custom"); err != nil {
		t.Errorf("custom copy not saved: %v", err)
	}
	if rec := e.send("GET", "/api/layouts", ""); rec.Code != 200 {
		t.Errorf("list: %d", rec.Code)
	}
}

func TestLayoutReplySaysEditAlways(t *testing.T) {
	e := newGenEnv(t, "")
	if r := decodeReply(t, e.send("GET", "/api/layout", "")); !r.Edit {
		t.Error("edit is false")
	}
}

// every guard still refuses, with no --edit.
func TestGuardsHoldWithoutEditFlag(t *testing.T) {
	e := newLayoutEnv(t)
	good := string(encode(t, "mine"))
	big := `{"v":1,"pad":"` + strings.Repeat("x", layout.MaxBytes+10) + `"}`
	cases := map[string]struct {
		body string
		opt  func(*http.Request)
		want int
	}{
		"no token":       {good, noCookie, 401},
		"wrong host":     {good, func(r *http.Request) { r.Host = "evil.test:4242" }, 403},
		"no edit header": {good, drop("X-Clalt-Edit"), 403},
		"foreign origin": {good, set("Origin", "http://evil.test"), 403},
		"foreign referer": {good, func(r *http.Request) {
			r.Header.Del("Origin")
			r.Header.Set("Referer", "http://evil.test/x")
		}, 403},
		"content type": {good, set("Content-Type", "text/plain"), 415},
		"too big":      {big, nil, 413},
		"invalid":      {`{"v":1,"extra":1}`, nil, 400},
		"template":     {`{"v":2,"template":"rows"}`, nil, 400},
	}
	for name, c := range cases {
		var opts []func(*http.Request)
		if c.opt != nil {
			opts = append(opts, c.opt)
		}
		if rec := e.send("PUT", "/api/layout", c.body, opts...); rec.Code != c.want {
			t.Errorf("%s: %d, want %d", name, rec.Code, c.want)
		}
	}
	if _, err := e.store.Load("mine"); err == nil {
		t.Error("a refused write was saved")
	}
}

func TestListShowsKindsAndTheActiveOne(t *testing.T) {
	e := newGenEnv(t, "")
	e.send("POST", "/api/layout/use", `{"name":"river"}`)
	rec := e.send("GET", "/api/layouts", "")
	var got struct {
		Active    string
		Templates []struct {
			ID, Title, Kind string
			Active          bool
		}
		Layouts []struct {
			Name, Title, Kind string
			Builtin, Active   bool
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Active != "river" || len(got.Templates) != 5 || len(got.Layouts) != 1 {
		t.Fatalf("%s", rec.Body)
	}
	for _, tp := range got.Templates {
		if tp.Kind != "template" || tp.Active != (tp.ID == "river") {
			t.Errorf("template %+v", tp)
		}
	}
	if l := got.Layouts[0]; l.Kind != "saved" || l.Active || l.Name != "default" {
		t.Errorf("layout %+v", l)
	}
}
