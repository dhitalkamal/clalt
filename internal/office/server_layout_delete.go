package office

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// layoutDelete removes a saved layout and answers with the new list. a built-in
// template is refused. when the layout is the active one, active moves to the
// default template first, so a failure never leaves it on a missing file.
func (h *handler) layoutDelete(w http.ResponseWriter, body []byte) {
	var req struct {
		Name string `json:"name"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || !layout.ValidName(req.Name) {
		http.Error(w, "body must be {\"name\": \"<layout name>\"}", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	active := h.activeNameLocked()
	moved := false
	if req.Name == active && !layout.IsTemplate(req.Name) {
		if err := h.store.SetActive(layout.DefaultActive); err != nil {
			h.mu.Unlock()
			h.logf("switching away from layout %s failed: %v", req.Name, err)
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		moved = true
	}
	err := h.store.Delete(req.Name)
	if err != nil && moved {
		// put active back when the layout still loads, else the default stays
		_ = h.store.SetActive(req.Name)
	}
	if err == nil && moved {
		h.start = ""
		active = layout.DefaultActive
	}
	h.mu.Unlock()
	switch {
	case errors.Is(err, layout.ErrBuiltin):
		http.Error(w, "built-in layouts cannot be deleted", http.StatusForbidden)
	case errors.Is(err, layout.ErrNotFound):
		http.Error(w, "no such layout", http.StatusNotFound)
	case err != nil:
		h.logf("deleting layout %s failed: %v", req.Name, err)
		http.Error(w, "delete failed", http.StatusInternalServerError)
	default:
		h.replyList(w, active)
	}
}
