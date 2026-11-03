// Package cfaccess lets the office trust Cloudflare Access as the login for
// one public hostname, so a tunnelled office needs no ?t= token. it checks the
// Cf-Access-Jwt-Assertion header that Access adds to every request it lets
// through, and refuses anything it cannot prove.
package cfaccess

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dhitalkamal/clalt/internal/fsutil"
)

// Config is access.json in the office dir. no file means the feature is off.
type Config struct {
	// Host is the public hostname the tunnel serves, no scheme or port
	Host string `json:"host"`
	// Team is the zero trust team name, as in <team>.cloudflareaccess.com
	Team string `json:"team"`
	// Aud is the application audience tag from the access app overview
	Aud string `json:"aud"`
	// Email is the one identity let in, even if the access policy is wider
	Email string `json:"email"`
}

const configName = "access.json"

// team ends up in the certs url, so it must stay a single dns label
var teamRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

var hostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// LoadConfig reads dir/access.json. on is false when there is no file or the
// file is unusable, so a broken config never opens anything.
func LoadConfig(dir string) (c Config, on bool, err error) {
	raw, err := fsutil.ReadFile(filepath.Join(dir, configName))
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, false, fmt.Errorf("access: %s is broken: %w", configName, err)
	}
	c.Host = strings.ToLower(strings.TrimSpace(c.Host))
	c.Team = strings.ToLower(strings.TrimSpace(c.Team))
	c.Aud = strings.TrimSpace(c.Aud)
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	if err := c.check(); err != nil {
		return Config{}, false, fmt.Errorf("access: %s: %w", configName, err)
	}
	return c, true, nil
}

func (c Config) check() error {
	switch {
	case !hostRe.MatchString(c.Host):
		return fmt.Errorf("host %q must be a plain public hostname", c.Host)
	case !teamRe.MatchString(c.Team):
		return fmt.Errorf("team %q must be the name before .cloudflareaccess.com", c.Team)
	case c.Aud == "":
		return errors.New("aud is required")
	case !strings.Contains(c.Email, "@"):
		return errors.New("email is required")
	}
	return nil
}

// Issuer is the iss claim access signs with for this team.
func (c Config) Issuer() string { return "https://" + c.Team + ".cloudflareaccess.com" }

func (c Config) certsURL() string { return c.Issuer() + "/cdn-cgi/access/certs" }
