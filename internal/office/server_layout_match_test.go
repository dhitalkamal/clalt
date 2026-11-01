package office

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// titled is a body of the minimal layout with a title.
func titled(t *testing.T, title string) string {
	t.Helper()
	l := layout.Minimal()
	l.Title = title
	return string(mustEncode(t, l))
}

func TestLayoutGetAndSaveCarryAVersion(t *testing.T) {
	e := newLayoutEnv(t)
	if got := e.send("GET", "/api/layout", "").Header().Get("ETag"); got != `"none"` {
		t.Fatalf("nothing stored: etag %q", got)
	}
	put := e.send("PUT", "/api/layout", titled(t, "One"), set("If-Match", `"none"`))
	v := put.Header().Get("ETag")
	if put.Code != 200 || v == "" || v == `"none"` {
		t.Fatalf("save %d etag %q %s", put.Code, v, put.Body)
	}
	if got := e.send("GET", "/api/layout", "").Header().Get("ETag"); got != v {
		t.Errorf("get etag %q, save etag %q", got, v)
	}
}

func TestLayoutSaveOnAStaleVersionIsRefusedWithTheStoredLayout(t *testing.T) {
	e := newLayoutEnv(t)
	first := e.send("PUT", "/api/layout", titled(t, "Tab A title"), set("If-Match", `"none"`))
	if first.Code != 200 {
		t.Fatalf("first %d %s", first.Code, first.Body)
	}
	stored, _ := e.store.StoredBytes(e.store.ActiveName())
	rec := e.send("PUT", "/api/layout", titled(t, "Tab B title"), set("If-Match", `"none"`))
	if rec.Code != 409 {
		t.Fatalf("stale save = %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Layout  layout.Layout `json:"layout"`
		Version string        `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Layout.Title != "Tab A title" || `"`+body.Version+`"` != first.Header().Get("ETag") || rec.Header().Get("ETag") != first.Header().Get("ETag") {
		t.Errorf("body %s", rec.Body)
	}
	if after, _ := e.store.StoredBytes(e.store.ActiveName()); string(after) != string(stored) {
		t.Error("a refused save changed the file")
	}
	again := e.send("PUT", "/api/layout", titled(t, "Tab B title"), set("If-Match", first.Header().Get("ETag")))
	if again.Code != 200 {
		t.Errorf("save on the current version = %d %s", again.Code, again.Body)
	}
}

func TestLayoutSaveWithoutIfMatchStillSaves(t *testing.T) {
	e := newLayoutEnv(t)
	e.send("PUT", "/api/layout", titled(t, "One"))
	if rec := e.send("PUT", "/api/layout", titled(t, "Two")); rec.Code != 200 {
		t.Errorf("no if-match = %d", rec.Code)
	}
}

func TestLayoutGuardsRunBeforeTheVersionCheck(t *testing.T) {
	e := newLayoutEnv(t)
	if rec := e.send("PUT", "/api/layout", titled(t, "x"), set("If-Match", `"stale"`), drop("X-Clalt-Edit")); rec.Code != 403 {
		t.Errorf("no edit header = %d", rec.Code)
	}
	if rec := e.send("PUT", "/api/layout", "nope", set("If-Match", `"stale"`)); rec.Code != 400 {
		t.Errorf("bad body = %d", rec.Code)
	}
}

func TestLayoutUseAnswersTheVersionOfThatLayout(t *testing.T) {
	e := newLayoutEnv(t)
	put := e.send("PUT", "/api/layout", titled(t, "One"))
	saved := e.store.ActiveName()
	rec := e.send("POST", "/api/layout/use", `{"name":"loft"}`)
	if rec.Code != 200 || rec.Header().Get("ETag") != `"none"` {
		t.Fatalf("use loft %d etag %q", rec.Code, rec.Header().Get("ETag"))
	}
	rec = e.send("POST", "/api/layout/use", `{"name":"`+saved+`"}`)
	if rec.Header().Get("ETag") != put.Header().Get("ETag") {
		t.Errorf("use etag %q, save etag %q", rec.Header().Get("ETag"), put.Header().Get("ETag"))
	}
}

func TestLayoutConcurrentSavesOnOneVersionLetOneWin(t *testing.T) {
	e := newLayoutEnv(t)
	const n = 10
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes <- e.send("PUT", "/api/layout", titled(t, strings.Repeat("t", i+1)), set("If-Match", `"none"`)).Code
		}(i)
	}
	wg.Wait()
	close(codes)
	ok, conflict := 0, 0
	for c := range codes {
		switch c {
		case 200:
			ok++
		case 409:
			conflict++
		}
	}
	if ok != 1 || conflict != n-1 {
		t.Errorf("%d saved, %d refused", ok, conflict)
	}
}
