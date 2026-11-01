package office

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// Generator builds the layout of a garden template (rows, village, river,
// street) from what this machine has now. rev changes only when the layout
// does, so the page can skip a rebuild. the hall is never asked for.
type Generator interface {
	Layout(template string) (l layout.Layout, rev string, err error)
}

// layoutReply is what the layout routes answer: edit (always true, kept for the
// page), the layout, the template list, the name that was answered (a template id or a
// saved layout) and the rev of the layout.
type layoutReply struct {
	Edit      bool              `json:"edit"`
	Layout    layout.Layout     `json:"layout"`
	Templates []layout.Template `json:"templates"`
	Active    string            `json:"active"`
	Rev       string            `json:"rev"`
}

// revOf is a short hash of a layout, the rev of everything not generated.
func revOf(l layout.Layout) string {
	raw, err := json.Marshal(l)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// activeName is the start template of this run while there is one, else the
// name in active.json.
func (h *handler) activeName() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.activeNameLocked()
}

// activeNameLocked is activeName for a caller that holds h.mu.
func (h *handler) activeNameLocked() string {
	if h.start != "" {
		return h.start
	}
	return h.store.ActiveName()
}

// resolve is the layout a name stands for: a generated garden, the hall, or a
// stored layout. it never fails: whatever cannot be had is logged and the hall
// answers instead.
func (h *handler) resolve(name string) (layout.Layout, string) {
	if layout.IsTemplate(name) && name != layout.HallTemplate && h.cfg.Generator != nil {
		l, rev, err := h.cfg.Generator.Layout(name)
		if err == nil {
			err = h.store.Check(l)
		}
		if err == nil {
			return l, rev
		}
		h.logf("generating template %s failed, showing the hall: %v", name, err)
	}
	if !layout.IsTemplate(name) {
		l, err := h.store.Load(name)
		if err == nil {
			return l, revOf(l)
		}
		h.logf("active layout %s is not usable (%v), showing the hall", name, err)
	}
	l := h.store.Default()
	return l, revOf(l)
}

// layoutGet answers a read: the template of the query when there is one, else
// the active one, in watch mode too.
func (h *handler) layoutGet(w http.ResponseWriter, r *http.Request) {
	name := ""
	if q := r.URL.Query(); q.Has("template") {
		name = q.Get("template")
		if !layout.IsTemplate(name) {
			http.Error(w, "unknown template", http.StatusBadRequest)
			return
		}
	} else {
		name = h.activeName()
	}
	l, rev := h.resolve(name)
	h.replyLayout(w, name, l, rev)
}

func (h *handler) replyLayout(w http.ResponseWriter, name string, l layout.Layout, rev string) {
	w.Header().Set("X-Clalt-Rev", rev)
	setVersion(w, h.layoutVersion(h.saveTarget(name, l.V)))
	h.reply(w, layoutReply{Edit: true, Layout: l, Templates: layout.Templates, Active: name, Rev: rev})
}

func (h *handler) reply(w http.ResponseWriter, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		h.logf("layout encoding failed: %v", err)
		http.Error(w, "layout failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}
