package office

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// fakeGen is a Generator that answers a small v2 layout named after the
// template, with a rev that says which template it was.
type fakeGen struct {
	mu    sync.Mutex
	calls []string
	fail  error
}

func (g *fakeGen) Layout(template string) (layout.Layout, string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, template)
	if g.fail != nil {
		return layout.Layout{}, "", g.fail
	}
	sq := []layout.Pt{{-9, -9}, {9, -9}, {9, 9}, {-9, 9}}
	l := layout.Layout{
		V: layout.Version, Name: template, Title: "Fake " + template, Template: template,
		Site: &layout.Site{
			Base: layout.Poly{Poly: sq}, Land: []layout.Poly{{Poly: sq}}, Trees: layout.Trees{Mode: "edge", Band: 3},
		},
		Pavilions: []layout.Pavilion{{ID: "lobby", Type: "lobby"}},
	}
	return l, "r-" + template, nil
}

func (g *fakeGen) called() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.calls...)
}

type genEnv struct {
	layoutEnv
	gen  *fakeGen
	logs *[]string
}

func newGenEnv(t *testing.T, edit bool, startTemplate string) genEnv {
	t.Helper()
	st := layout.NewStore(t.TempDir(), nil)
	st.Catalog = layout.Catalog{}
	st.Builtin = fstest.MapFS{"default.json": {Data: encode(t, "default")}}
	gen := &fakeGen{}
	var logs []string
	h, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Edit: edit, Layouts: st,
		Generator: gen, Template: startTemplate,
		Logf:     func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) },
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return genEnv{layoutEnv{h, st}, gen, &logs}
}

func TestGetLayoutWithoutQueryAnswersTheActiveTemplate(t *testing.T) {
	e := newGenEnv(t, false, "")
	rec := e.send("GET", "/api/layout", "")
	r := decodeReply(t, rec)
	if rec.Code != 200 || r.Layout.V != 2 || r.Layout.Name != "rows" || r.Rev != "r-rows" || r.Active != "rows" {
		t.Fatalf("%d %+v", rec.Code, r)
	}
	var ids []string
	for _, tp := range r.Templates {
		ids = append(ids, tp.ID+"="+tp.Title)
	}
	if got := strings.Join(ids, ","); got != "rows=Rows,village=Village,river=River,street=Street,hall=Hall" {
		t.Errorf("templates = %s", got)
	}
	if rec.Header().Get("X-Clalt-Rev") != "r-rows" {
		t.Errorf("rev header = %q", rec.Header().Get("X-Clalt-Rev"))
	}
}

func TestGetLayoutTemplateQuery(t *testing.T) {
	e := newGenEnv(t, false, "")
	r := decodeReply(t, e.send("GET", "/api/layout?template=river", ""))
	if r.Layout.Name != "river" || r.Rev != "r-river" || r.Active != "river" {
		t.Errorf("reply %+v", r)
	}
	if e.store.ActiveName() != layout.DefaultActive {
		t.Error("a read changed the active layout")
	}
	if _, err := e.store.Load("river"); err == nil {
		t.Error("a read stored the template")
	}
}

func TestGetLayoutHallTemplateIsTheDefaultFileWithoutTheGenerator(t *testing.T) {
	e := newGenEnv(t, false, "")
	a := decodeReply(t, e.send("GET", "/api/layout?template=hall", ""))
	b := decodeReply(t, e.send("GET", "/api/layout?template=hall", ""))
	if a.Layout.V != 1 || a.Layout.Name != "default" || a.Rev == "" || a.Rev != b.Rev {
		t.Errorf("hall reply %+v / %+v", a, b)
	}
	if len(e.gen.called()) != 0 {
		t.Errorf("the generator was asked for the hall: %v", e.gen.called())
	}
}

func TestGetLayoutUnknownTemplateIs400(t *testing.T) {
	e := newGenEnv(t, false, "")
	for _, q := range []string{"nope", "..%2Fx", "default", "mine", "ROWS", "rows%00"} {
		if rec := e.send("GET", "/api/layout?template="+q, ""); rec.Code != 400 {
			t.Errorf("template=%s = %d", q, rec.Code)
		}
	}
	if len(e.gen.called()) != 0 {
		t.Errorf("the generator ran for a bad name: %v", e.gen.called())
	}
}

func TestGetLayoutTemplateQueryNeedsTheToken(t *testing.T) {
	e := newGenEnv(t, false, "")
	if rec := e.send("GET", "/api/layout?template=river", "", noCookie); rec.Code != 401 {
		t.Errorf("no cookie = %d", rec.Code)
	}
}

func TestGeneratorErrorFallsBackToTheHall(t *testing.T) {
	e := newGenEnv(t, false, "")
	e.gen.fail = errors.New("boom")
	rec := e.send("GET", "/api/layout?template=village", "")
	r := decodeReply(t, rec)
	if rec.Code != 200 || r.Layout.V != 1 || r.Layout.Name != "default" || r.Rev == "" {
		t.Fatalf("%d %+v", rec.Code, r)
	}
	if len(*e.logs) == 0 || !strings.Contains((*e.logs)[0], "boom") {
		t.Errorf("logs = %v", *e.logs)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("the error text reached the page")
	}
}

func TestNoGeneratorServesTheHall(t *testing.T) {
	e := newLayoutEnv(t, false)
	r := decodeReply(t, e.send("GET", "/api/layout?template=rows", ""))
	if r.Layout.Name != "default" || r.Layout.V != 1 || len(r.Templates) != 5 || r.Rev == "" {
		t.Errorf("reply %+v", r)
	}
}

func TestActiveJSONMayNameASavedLayout(t *testing.T) {
	e := newGenEnv(t, false, "")
	l := layout.Minimal()
	l.Name, l.Title = "mine", "Mine"
	if err := e.store.Save(l); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetActive("mine"); err != nil {
		t.Fatal(err)
	}
	r := decodeReply(t, e.send("GET", "/api/layout", ""))
	if r.Layout.Name != "mine" || r.Active != "mine" || len(e.gen.called()) != 0 {
		t.Errorf("reply %+v calls %v", r, e.gen.called())
	}
}

func TestStartTemplateOverridesActiveWithoutWriting(t *testing.T) {
	e := newGenEnv(t, true, "village")
	if r := decodeReply(t, e.send("GET", "/api/layout", "")); r.Layout.Name != "village" {
		t.Errorf("start template ignored: %+v", r.Layout.Name)
	}
	if e.store.ActiveName() != layout.DefaultActive {
		t.Error("the start template was written to active.json")
	}
	// the query still wins for that read
	if r := decodeReply(t, e.send("GET", "/api/layout?template=street", "")); r.Layout.Name != "street" {
		t.Errorf("query = %q", r.Layout.Name)
	}
	// a saved default from the page replaces the start template
	if rec := e.send("POST", "/api/layout/use", `{"name":"river"}`); rec.Code != 200 {
		t.Fatalf("use = %d %s", rec.Code, rec.Body)
	}
	if r := decodeReply(t, e.send("GET", "/api/layout", "")); r.Layout.Name != "river" {
		t.Errorf("after use = %q", r.Layout.Name)
	}
}

func TestBadStartTemplateIsRefused(t *testing.T) {
	_, err := NewHandler(Config{
		Token: testToken, Port: 4242, Hosts: []string{"127.0.0.1:4242"}, Template: "castle",
		Snapshot: func() (Snapshot, error) { return Snapshot{V: 1}, nil },
	})
	if err == nil {
		t.Error("a start template that is not in the list was accepted")
	}
}

func TestUseAcceptsTemplateNames(t *testing.T) {
	e := newGenEnv(t, true, "")
	for _, name := range []string{"river", "hall"} {
		rec := e.send("POST", "/api/layout/use", fmt.Sprintf(`{"name":%q}`, name))
		r := decodeReply(t, rec)
		if rec.Code != 200 || e.store.ActiveName() != name || r.Rev == "" || len(r.Templates) != 5 {
			t.Errorf("use %s: %d %+v active %q", name, rec.Code, r, e.store.ActiveName())
		}
	}
	if rec := e.send("POST", "/api/layout/use", `{"name":"river"}`); decodeReply(t, rec).Layout.V != 2 {
		t.Error("use river did not answer the generated layout")
	}
	if rec := e.send("POST", "/api/layout/use", `{"name":"nope"}`); rec.Code != 404 {
		t.Errorf("unknown = %d", rec.Code)
	}
	if rec := e.send("POST", "/api/layout/use", `{"name":"river"}`, drop("X-Clalt-Edit")); rec.Code != 403 {
		t.Errorf("guards lost: %d", rec.Code)
	}
}

func TestPutRefusesATemplateLayout(t *testing.T) {
	e := newGenEnv(t, true, "")
	l := layout.Layout{V: 2, Name: "x", Title: "X", Template: "river"}
	raw, _ := layout.Encode(l)
	rec := e.send("PUT", "/api/layout", string(raw))
	if rec.Code != 400 {
		t.Errorf("PUT of a template layout = %d %s", rec.Code, rec.Body)
	}
	if e.store.ActiveName() != layout.DefaultActive {
		t.Error("active changed")
	}
}

func TestPutHallEditWhileATemplateIsActiveSavesHallCustom(t *testing.T) {
	e := newGenEnv(t, true, "")
	l := layout.Minimal()
	l.Title = "Edited"
	raw, _ := layout.Encode(l)
	rec := e.send("PUT", "/api/layout", string(raw))
	if rec.Code != 200 {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	if e.store.ActiveName() != "hall-custom" {
		t.Errorf("active = %q", e.store.ActiveName())
	}
	if got, err := e.store.Load("hall-custom"); err != nil || got.Title != "Edited" {
		t.Errorf("saved = %+v, %v", got, err)
	}
	if r := decodeReply(t, rec); len(r.Templates) != 5 || r.Rev == "" {
		t.Errorf("PUT reply %+v", r)
	}
}

func TestListRepliesWithTemplates(t *testing.T) {
	e := newGenEnv(t, true, "")
	rec := e.send("GET", "/api/layouts", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"templates"`) {
		t.Errorf("list = %d %s", rec.Code, rec.Body)
	}
}
