package office

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	tokenHeader = "X-Clalt-Token"
)

// cookieNameFor is per port, so two offices running at once do not overwrite
// each other's cookie (cookies are shared across ports of one host).
func cookieNameFor(port int) string { return "clalt_office_" + strconv.Itoa(port) }

// Config is what the handler needs. Hosts are the Host header values the
// server answers to, the rest is refused. Logf is where a failure goes, the
// response never says why.
type Config struct {
	Token    string
	Port     int // the listen port, names the cookie
	Hosts    []string
	Snapshot func() (Snapshot, error)
	Logf     func(format string, args ...any)
}

type handler struct {
	cfg   Config
	hosts map[string]bool
	web   fs.FS
	csp   string
}

var importMap = regexp.MustCompile(`(?s)<script type="importmap">(.*?)</script>`)

// contentSecurityPolicy allows the page's own files and nothing else. the page
// needs an import map, which is inline, so its hash is allowed and no
// unsafe-inline is.
func contentSecurityPolicy(page []byte) string {
	script := "script-src 'self'"
	if m := importMap.FindSubmatch(page); m != nil {
		sum := sha256.Sum256(m[1])
		script += " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	return strings.Join([]string{
		"default-src 'none'", script, "style-src 'self'", "img-src 'self'", "connect-src 'self'", "font-src 'self'",
		"frame-ancestors 'none'", "base-uri 'none'", "form-action 'none'",
	}, "; ")
}

// NewHandler builds the handler of the office: token, host and method checks
// first, then the snapshot or a file of the embed. no CORS header is ever set.
func NewHandler(cfg Config) (http.Handler, error) {
	if cfg.Token == "" || len(cfg.Hosts) == 0 || cfg.Snapshot == nil {
		return nil, errors.New("office: handler needs a token, hosts and a snapshot source")
	}
	web := webFS()
	page, err := fs.ReadFile(web, "index.html")
	if err != nil {
		return nil, err
	}
	h := &handler{cfg: cfg, hosts: map[string]bool{}, web: web, csp: contentSecurityPolicy(page)}
	for _, host := range cfg.Hosts {
		h.hosts[strings.ToLower(host)] = true
	}
	return h, nil
}

func (h *handler) logf(format string, args ...any) {
	if h.cfg.Logf != nil {
		h.cfg.Logf(format, args...)
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hd := w.Header()
	hd.Set("Content-Security-Policy", h.csp)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Referrer-Policy", "no-referrer")
	hd.Set("Cross-Origin-Resource-Policy", "same-origin")
	// a name that resolves to this machine from another site would pass the token check by cookie, so the Host must be ours
	if !h.hosts[strings.ToLower(r.Host)] {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		hd.Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if q := r.URL.Query().Get("t"); q != "" && h.tokenOK(q) {
		http.SetCookie(w, &http.Cookie{Name: cookieNameFor(h.cfg.Port), Value: h.cfg.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		hd.Set("Cache-Control", "no-store")
		// a constant target: building it from the request path would let //host/x redirect off site
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.URL.Path == "/api/snapshot" {
		h.snapshot(w)
		return
	}
	h.file(w, r)
}

func (h *handler) tokenOK(got string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.cfg.Token)) == 1
}

// authorized is the token in the header or the cookie. the query token is only
// good for the first load, see ServeHTTP.
func (h *handler) authorized(r *http.Request) bool {
	if t := r.Header.Get(tokenHeader); t != "" && h.tokenOK(t) {
		return true
	}
	c, err := r.Cookie(cookieNameFor(h.cfg.Port))
	return err == nil && h.tokenOK(c.Value)
}

func (h *handler) snapshot(w http.ResponseWriter) {
	snap, err := h.cfg.Snapshot()
	if err != nil {
		h.logf("snapshot failed: %v", err)
		http.Error(w, "snapshot failed", http.StatusInternalServerError)
		return
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		h.logf("snapshot encoding failed: %v", err)
		http.Error(w, "snapshot failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}

var types = map[string]string{
	".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8",
	".txt": "text/plain; charset=utf-8", ".json": "application/json", ".png": "image/png", ".glb": "model/gltf-binary",
}

// file serves one file of the embed. anything that is not a clean path to a
// regular file in it is a 404, so nothing outside the embed can be named.
func (h *handler) file(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	if !fs.ValidPath(name) || strings.Contains(name, `\`) {
		http.NotFound(w, r)
		return
	}
	info, err := fs.Stat(h.web, name)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	raw, err := fs.ReadFile(h.web, name)
	if err != nil {
		h.logf("reading embedded %s failed: %v", name, err)
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	ct := types[strings.ToLower(path.Ext(name))]
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}
