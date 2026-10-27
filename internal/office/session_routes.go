package office

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

// SessionActions is what the page can ask of a session. every action goes
// through clalt's own paths: a move request for the wrapper, the terminal
// driver, a stop request for the wrapper. a refusal is a *RefusedError.
type SessionActions interface {
	Move(handle, account string) error
	Focus(ctx context.Context, handle string) error
	Stop(handle string) error
}

// RefusedError is an action that cannot be done, with a reason fit for the page.
type RefusedError struct{ Reason string }

func (e *RefusedError) Error() string { return e.Reason }

// maxSessionBody is plenty for a handle and an account id.
const maxSessionBody = 2048

// focusTimeout bounds the call to kitty or tmux.
const focusTimeout = 5 * time.Second

var (
	handleRe  = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
	accountRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

type sessionBody struct {
	Handle  string `json:"handle"`
	Account string `json:"account"`
}

// sessionRoute handles POST /api/session/<move|focus|stop>. the order is the
// one of the layout writes: token, method, then the edit header, origin and
// content type (writeAllowed), the body size, and last the checks of the body.
func (h *handler) sessionRoute(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	verb := strings.TrimPrefix(r.URL.Path, "/api/session/")
	if h.cfg.Actions == nil || (verb != "move" && verb != "focus" && verb != "stop") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.writeAllowed(w, r) {
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSessionBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "request is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "could not read the body", http.StatusBadRequest)
		}
		return
	}
	var body sessionBody
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&body) != nil || dec.More() || !handleRe.MatchString(body.Handle) || (verb == "move" && !accountRe.MatchString(body.Account)) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// a plain claude session has no wrapper to ask: nothing is tried
	if feed.IsExt(body.Handle) {
		writeJSON(w, http.StatusConflict, map[string]string{"reason": outsideWhy})
		return
	}
	h.runAction(w, r, verb, body)
}

func (h *handler) runAction(w http.ResponseWriter, r *http.Request, verb string, b sessionBody) {
	var err error
	ok, code := map[string]any{"queued": true}, http.StatusAccepted
	switch verb {
	case "move":
		err = h.cfg.Actions.Move(b.Handle, b.Account)
	case "focus":
		ctx, cancel := context.WithTimeout(r.Context(), focusTimeout)
		defer cancel()
		err = h.cfg.Actions.Focus(ctx, b.Handle)
		ok, code = map[string]any{"focused": true}, http.StatusOK
	case "stop":
		err = h.cfg.Actions.Stop(b.Handle)
		ok = map[string]any{"stopping": true}
	}
	var refused *RefusedError
	switch {
	case errors.As(err, &refused):
		writeJSON(w, http.StatusConflict, map[string]string{"reason": clean(refused.Reason, 240)})
	case err != nil:
		h.logf("session %s failed: %v", verb, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"reason": "that did not work, the office log says why"})
	default:
		writeJSON(w, code, ok)
	}
}
