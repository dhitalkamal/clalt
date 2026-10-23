package cli

import (
	"fmt"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/usage"
)

// oneAccountArgs reads the arguments of a command that takes one account and
// nothing else.
func oneAccountArgs(args []string, usage string) (string, error) {
	var label string
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-"):
			return "", usageError{"unknown flag " + arg, usage}
		case label != "":
			return "", usageError{"unexpected argument " + arg, usage}
		default:
			label = arg
		}
	}
	if label == "" {
		return "", usageError{"which account? give it a name", usage}
	}
	return label, nil
}

// refuseReservedLabel is the error for the label that usage reports use for the
// usage that no account can claim. an account with that label would be taken for
// it. labels are compared ignoring case everywhere, so this one is too.
func refuseReservedLabel(label string) error {
	if !strings.EqualFold(label, usage.Unattributed) {
		return nil
	}
	return fmt.Errorf("the label %q is reserved (in any case): usage reports use %q for usage that no account can claim. choose another label",
		label, usage.Unattributed)
}

// findAccount is the visible or removed account a label refers to.
func (a *App) findAccount(label string) (account.Account, error) {
	acc, found, err := a.Store.ResolveLabel(label)
	if err != nil {
		return acc, err
	}
	if !found {
		return acc, fmt.Errorf("no account called %q", label)
	}
	return acc, nil
}

// findVisible is an account that was not removed.
func (a *App) findVisible(label string) (account.Account, error) {
	acc, err := a.findAccount(label)
	if err != nil {
		return acc, err
	}
	if acc.Meta.Hidden {
		return acc, fmt.Errorf("%q was removed. 'clalt login %s' brings it back", label, label)
	}
	return acc, nil
}
