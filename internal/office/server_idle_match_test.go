package office

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const idleDesk = `{"default":"break","teamDefault":"desk","accounts":{"work":"desk"}}`

func ifMatch(v string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("If-Match", v) }
}

func TestIdleGetCarriesAVersion(t *testing.T) {
	e := newIdleEnv(t)
	if got := e.do("GET", "").Header().Get("ETag"); got != `"none"` {
		t.Fatalf("no file: etag %q", got)
	}
	post := e.do("POST", idleDesk)
	v := post.Header().Get("ETag")
	if post.Code != 200 || v == "" || v == `"none"` {
		t.Fatalf("save: %d etag %q", post.Code, v)
	}
	if got := e.do("GET", "").Header().Get("ETag"); got != v {
		t.Errorf("get etag %q, save etag %q", got, v)
	}
}

func TestIdlePostWithAStaleVersionIsRefusedWithTheCurrentRules(t *testing.T) {
	e := newIdleEnv(t)
	v0 := e.do("GET", "").Header().Get("ETag")
	if rec := e.do("POST", idleDesk, ifMatch(v0)); rec.Code != 200 {
		t.Fatalf("first save %d %s", rec.Code, rec.Body)
	}
	before, _ := os.ReadFile(filepath.Join(e.dir, "idle.json"))
	rec := e.do("POST", `{"default":"hide","teamDefault":"desk"}`, ifMatch(v0))
	if rec.Code != 409 {
		t.Fatalf("stale save = %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Rules   IdleRules `json:"rules"`
		Version string    `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Rules.Accounts["work"] != "desk" || body.Version == "" || `"`+body.Version+`"` != rec.Header().Get("ETag") {
		t.Errorf("body %s etag %q", rec.Body, rec.Header().Get("ETag"))
	}
	after, _ := os.ReadFile(filepath.Join(e.dir, "idle.json"))
	if string(before) != string(after) {
		t.Error("a refused save changed the file")
	}
}

func TestIdlePostWithoutIfMatchStillSaves(t *testing.T) {
	e := newIdleEnv(t)
	e.do("POST", idleDesk)
	if rec := e.do("POST", `{"default":"hide","teamDefault":"desk"}`); rec.Code != 200 {
		t.Errorf("no if-match = %d", rec.Code)
	}
}

func TestIdleGuardsRunBeforeTheVersionCheck(t *testing.T) {
	e := newIdleEnv(t)
	if rec := e.do("POST", idleDesk, ifMatch(`"stale"`), func(r *http.Request) { r.Header.Del("X-Clalt-Edit") }); rec.Code != 403 {
		t.Errorf("no edit header = %d", rec.Code)
	}
	if rec := e.do("POST", `nope`, ifMatch(`"stale"`)); rec.Code != 400 {
		t.Errorf("bad body = %d", rec.Code)
	}
}

func TestIdleConcurrentSavesOnOneVersionLetOneWin(t *testing.T) {
	e := newIdleEnv(t)
	const n = 12
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := `{"default":"break","teamDefault":"desk","accounts":{"a` + strings.Repeat("x", i) + `":"hide"}}`
			codes <- e.do("POST", body, ifMatch(`"none"`)).Code
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
