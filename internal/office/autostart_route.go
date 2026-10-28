package office

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
)

// Autostarter is start at login as the settings dialog sees it.
type Autostarter interface {
	Status() AutostartStatus
	Set(on bool) error
}

// AutostartStatus is the reply of GET and POST /api/office/autostart.
type AutostartStatus struct {
	Supported bool `json:"supported"`
	Enabled   bool `json:"enabled"`
}

const autostartPath = "/api/office/autostart"

// pingReply is what a server says to GET /api/ping, so a caller can tell a
// clalt office server of that pid from anything else on the port.
type pingReply struct {
	App     string `json:"app"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

// ping answers the handshake of clalt office off. the token check already ran.
func (h *handler) ping(w http.ResponseWriter) {
	h.writeJSON(w, pingReply{App: "clalt-office", PID: os.Getpid(), Version: h.cfg.Version})
}

func (h *handler) writeJSON(w http.ResponseWriter, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}

// autostartGet is the read route.
func (h *handler) autostartGet(w http.ResponseWriter) {
	if h.cfg.Autostart == nil {
		http.Error(w, "start at login is not available", http.StatusNotFound)
		return
	}
	h.writeJSON(w, h.cfg.Autostart.Status())
}

// autostartSave is the write route, after the guards of editRoute. the body is
// exactly {"enabled": bool}.
func (h *handler) autostartSave(w http.ResponseWriter, body []byte) {
	if h.cfg.Autostart == nil {
		http.Error(w, "start at login is not available", http.StatusNotFound)
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Enabled == nil || dec.More() {
		http.Error(w, `body must be {"enabled": true|false}`, http.StatusBadRequest)
		return
	}
	if !h.cfg.Autostart.Status().Supported {
		http.Error(w, "start at login is macOS only for now", http.StatusBadRequest)
		return
	}
	if err := h.cfg.Autostart.Set(*req.Enabled); err != nil {
		h.logf("start at login change failed: %v", err)
		http.Error(w, "could not change start at login", http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, h.cfg.Autostart.Status())
}
