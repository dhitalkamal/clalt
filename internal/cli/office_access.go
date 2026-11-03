package cli

import (
	"time"

	"github.com/dhitalkamal/clalt/internal/office/cfaccess"
)

// officeAccess turns on cloudflare access login when dir has an access.json.
// a broken file is logged and leaves it off, the token keeps working either way.
func officeAccess(dir string, logf func(string, ...any)) *cfaccess.Verifier {
	cfg, on, err := cfaccess.LoadConfig(dir)
	if err != nil {
		logf("access: off: %v", err)
		return nil
	}
	if !on {
		return nil
	}
	return cfaccess.NewVerifier(cfg, cfaccess.HTTPFetch, time.Now)
}
