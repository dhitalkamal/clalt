package office

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// noVersion is the version of a document that has no file yet.
const noVersion = "none"

// versionOf is a short hash of the bytes of a stored file, the version a
// save compares with If-Match.
func versionOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// setVersion puts the version on a reply as a strong ETag.
func setVersion(w http.ResponseWriter, v string) {
	w.Header().Set("ETag", `"`+v+`"`)
}

// staleWrite says whether the request names a version other than current.
// a request without If-Match is never stale, so clients that do not know
// about versions keep saving as before.
func staleWrite(r *http.Request, current string) bool {
	want, ok := r.Header["If-Match"]
	if !ok {
		return false
	}
	got := strings.TrimPrefix(strings.TrimSpace(strings.Join(want, ",")), "W/")
	return strings.Trim(got, `"`) != current
}

// conflict answers 409 with a JSON body: the current document and version.
func (h *handler) conflict(w http.ResponseWriter, body any) {
	raw, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "conflict", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusConflict)
	_, _ = w.Write(raw)
}
