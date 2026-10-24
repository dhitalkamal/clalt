package office

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

func TestListAndUse(t *testing.T) {
	e := newLayoutEnv(t, true)
	rec := e.send("GET", "/api/layouts", "")
	var list struct {
		Active  string
		Layouts []layout.Info
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != 200 {
		t.Fatalf("%d %v %s", rec.Code, err, rec.Body)
	}
	if list.Active != layout.DefaultActive || len(list.Layouts) != 2 || !list.Layouts[1].Builtin || list.Layouts[1].Name != "loft" {
		t.Errorf("list %+v", list)
	}

	rec = e.send("POST", "/api/layout/use", `{"name":"loft"}`)
	if rec.Code != 200 || decodeReply(t, rec).Layout.Name != "loft" || e.store.ActiveName() != "loft" {
		t.Errorf("use loft: %d %s", rec.Code, rec.Body)
	}
	if rec := e.send("POST", "/api/layout/use", `{"name":"nope"}`); rec.Code != 404 {
		t.Errorf("unknown = %d", rec.Code)
	}
	for _, body := range []string{`{"name":"../x"}`, `{"name":"a","x":1}`, `nope`, ``} {
		if rec := e.send("POST", "/api/layout/use", body); rec.Code != 400 {
			t.Errorf("%q = %d, want 400", body, rec.Code)
		}
	}
	if e.store.ActiveName() != "loft" {
		t.Error("a refused use changed the active layout")
	}
}

func TestSnapshotRouteIsUnchanged(t *testing.T) {
	e := newLayoutEnv(t, true)
	rec := e.send("GET", "/api/snapshot", "")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "layout") {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}
