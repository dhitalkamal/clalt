package office

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// editHeader must be sent with every write. a page of another origin cannot
// set it without a preflight, and the server answers no preflight.
const editHeader = "X-Clalt-Edit"

// isEditRoute says whether a request is for the write surface: PUT and the
// like on /api/layout, and everything on the two other routes. GET and HEAD
// of /api/layout are the read route.
func isEditRoute(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/layout":
		return r.Method != http.MethodGet && r.Method != http.MethodHead
	case "/api/layouts", "/api/layout/use":
		return true
	}
	return false
}

// editRoute is the order of the guards: the token, then whether there is a
// write surface at all (not without --edit), then the method, then for a write
// the edit header, the origin and the content type.
func (h *handler) editRoute(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.cfg.Edit {
		http.NotFound(w, r)
		return
	}
	want := http.MethodPost
	switch r.URL.Path {
	case "/api/layout":
		want = http.MethodPut
	case "/api/layouts":
		want = http.MethodGet
	}
	if r.Method != want {
		w.Header().Set("Allow", want)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if want == http.MethodGet {
		h.layoutList(w)
		return
	}
	if !h.writeAllowed(w, r) {
		return
	}
	body, ok := h.readBody(w, r)
	if !ok {
		return
	}
	if want == http.MethodPut {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.layoutPut(w, body)
		return
	}
	h.layoutUse(w, body)
}

// ownOrigin says whether an origin (scheme and host) is this server.
func (h *handler) ownOrigin(origin string) bool {
	host, ok := strings.CutPrefix(strings.ToLower(origin), "http://")
	return ok && h.hosts[host]
}

// writeAllowed is the write guard. the Origin header decides when there is
// one (a browser sends it on every write), else the Referer does. neither is
// a refusal.
func (h *handler) writeAllowed(w http.ResponseWriter, r *http.Request) bool {
	ok := r.Header.Get(editHeader) == "1"
	if ok {
		if o := r.Header.Get("Origin"); o != "" {
			ok = h.ownOrigin(o)
		} else if ref := r.Header.Get("Referer"); ref != "" {
			u, err := url.Parse(ref)
			ok = err == nil && h.ownOrigin(u.Scheme+"://"+u.Host)
		} else {
			ok = false
		}
	}
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	return true
}

// readBody reads a write body, at most one layout big.
func (h *handler) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, layout.MaxBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "layout is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "could not read the body", http.StatusBadRequest)
		}
		return nil, false
	}
	return body, true
}

func (h *handler) layoutList(w http.ResponseWriter) {
	infos, err := h.store.List()
	if err != nil {
		h.logf("listing layouts failed: %v", err)
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	h.reply(w, struct {
		Active    string            `json:"active"`
		Layouts   []layout.Info     `json:"layouts"`
		Templates []layout.Template `json:"templates"`
	}{h.activeName(), infos, layout.Templates})
}

// customName is the name a copy of a built-in template is saved under.
func customName(name string) string {
	const suffix = "-custom"
	if max := 40 - len(suffix); len(name) > max {
		name = strings.TrimRight(name[:max], "-")
	}
	return name + suffix
}

// layoutPut saves the body as the active layout. a built-in or generated
// template is never written: the copy is saved as "<name>-custom" and made
// active. a layout that names a template is refused, templates are generated.
func (h *handler) layoutPut(w http.ResponseWriter, body []byte) {
	l, err := layout.Decode(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if l.Template != "" {
		http.Error(w, "a layout that names a template is generated, never stored", http.StatusBadRequest)
		return
	}
	active := h.activeNameLocked()
	l.Name = active
	switch {
	case layout.IsTemplate(active):
		// a hall (v1) edit never lands on a garden template's name
		base := active
		if l.V != layout.Version {
			base = layout.HallTemplate
		}
		l.Name = customName(base)
	case h.store.IsBuiltin(active):
		l.Name = customName(active)
	}
	if err := h.store.Check(l); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.store.IsBuiltin(l.Name) {
		http.Error(w, layout.ErrBuiltin.Error(), http.StatusBadRequest)
		return
	}
	// a stored layout keeps its format: v1 over v2 (or back) would destroy it
	if old, err := h.store.Load(l.Name); err == nil && old.V != l.V {
		http.Error(w, "layout "+l.Name+" is format v"+strconv.Itoa(old.V)+", this edit is v"+strconv.Itoa(l.V)+"; it was not overwritten", http.StatusConflict)
		return
	}
	if err := h.store.Save(l); err != nil {
		h.logf("saving layout %s failed: %v", l.Name, err)
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	if l.Name != active {
		if err := h.store.SetActive(l.Name); err != nil {
			h.logf("switching to layout %s failed: %v", l.Name, err)
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
	}
	h.start = ""
	h.replyLayout(w, true, l.Name, l, revOf(l))
}

// layoutUse makes a stored layout the active one.
func (h *handler) layoutUse(w http.ResponseWriter, body []byte) {
	var req struct {
		Name string `json:"name"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || !layout.ValidName(req.Name) {
		http.Error(w, "body must be {\"name\": \"<layout name>\"}", http.StatusBadRequest)
		return
	}
	var l layout.Layout
	rev := ""
	// h.mu covers the switch only, a template is generated after it is let go
	h.mu.Lock()
	if !layout.IsTemplate(req.Name) {
		var err error
		l, err = h.store.Load(req.Name)
		switch {
		case errors.Is(err, layout.ErrNotFound):
			h.mu.Unlock()
			http.Error(w, "no such layout", http.StatusNotFound)
			return
		case err != nil:
			h.mu.Unlock()
			h.logf("layout %s does not load: %v", req.Name, err)
			http.Error(w, "that layout is not valid", http.StatusBadRequest)
			return
		}
		rev = revOf(l)
	}
	if err := h.store.SetActive(req.Name); err != nil {
		h.mu.Unlock()
		h.logf("switching to layout %s failed: %v", req.Name, err)
		http.Error(w, "switch failed", http.StatusInternalServerError)
		return
	}
	h.start = ""
	h.mu.Unlock()
	if layout.IsTemplate(req.Name) {
		l, rev = h.resolve(req.Name)
	}
	h.replyLayout(w, true, req.Name, l, rev)
}
