package office

import (
	"context"
	"net/http"
	"strings"

	"github.com/dhitalkamal/clalt/internal/office/cfaccess"
)

// accessKey marks a request whose cloudflare access assertion checked out.
// ServeHTTP sets it once so the host, token and write checks do not each
// verify the signature (and log the same refusal) again.
type accessKey struct{}

// withAccess verifies the assertion, only for the configured public host. any
// other host never gets this way in, so a stolen assertion is no use locally.
func (h *handler) withAccess(r *http.Request) *http.Request {
	v := h.cfg.Access
	if v == nil || strings.ToLower(r.Host) != v.Config().Host {
		return r
	}
	// the header only, not the CF_Authorization cookie: access sets the header
	// on the hop to the origin, a browser cannot forge one through the tunnel
	if err := v.Verify(r.Header.Get(cfaccess.Header)); err != nil {
		h.logf("access: refused %s %s: %v", r.Method, r.URL.Path, err)
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), accessKey{}, true))
}

func viaAccess(r *http.Request) bool {
	ok, _ := r.Context().Value(accessKey{}).(bool)
	return ok
}

// accessOrigin lets the public https origin write, and only with the assertion,
// so the page served through the tunnel can save but a stolen token cannot.
func (h *handler) accessOrigin(r *http.Request, origin string) bool {
	return viaAccess(r) && strings.EqualFold(origin, "https://"+h.cfg.Access.Config().Host)
}
