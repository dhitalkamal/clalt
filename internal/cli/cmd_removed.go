package cli

// goneError is a command or flag that no longer exists. it exits with 2 and
// prints one line, which says what to use instead.
type goneError struct{ msg string }

func (e goneError) Error() string { return e.msg }

const liveGone = "--live is gone: it needed the token vault. live limits return with the statusline feed"

var goneCommands = map[string]string{
	"add":       "clalt add is gone: use 'clalt login <name>'",
	"refresh":   "clalt refresh is gone: claude refreshes its own login, use 'clalt login <name>' if an account is signed out",
	"keepalive": "clalt keepalive is gone and no longer needed: 'clalt doctor' shows an old job that is still installed",
	"export":    "clalt export is gone: accounts hold no secrets now, so there is nothing to back up",
	"import":    "clalt import is gone: accounts hold no secrets now, so there is nothing to restore",
}

var goneFlags = map[string]map[string]string{
	"run": {
		"--refresh":      "run --refresh is gone: claude refreshes its own login",
		"--isolated":     "run --isolated is gone: every account shares your claude settings now",
		"--no-supervise": "run --no-supervise is gone: there is no restart supervisor any more",
		"--link":         "run --link is gone: sharing is always on",
	},
	"usage":     {"--live": liveGone},
	"dashboard": {"--live": liveGone},
	"doctor":    {"--fix": "doctor --fix is gone: doctor only reports now, 'clalt run' repairs links"},
	"login":     {"--magic-link": "login --magic-link is gone: claude signs in through its own browser flow now"},
}

// goneUse returns the refusal for a retired command, or for a retired flag of
// cmd among args. it runs before a command does anything, so a refused call has
// no side effects.
func goneUse(cmd string, args []string) error {
	if msg, ok := goneCommands[cmd]; ok {
		return goneError{msg}
	}
	for _, arg := range args {
		if arg == "--" {
			// what follows belongs to claude
			break
		}
		if msg, ok := goneFlags[cmd][arg]; ok {
			return goneError{msg}
		}
	}
	return nil
}
