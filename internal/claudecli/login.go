package claudecli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Login builds `claude auth login` for configDir, with --email when email is set so
// the browser form comes pre-filled. it does not attach stdio or start
// anything: the login is interactive, so the caller owns the terminal (the
// dashboard runs it through tea.ExecProcess).
func Login(ctx context.Context, configDir, email string) (*exec.Cmd, error) {
	if err := checkFolder(configDir); err != nil {
		return nil, fmt.Errorf("claude auth login: %w", err)
	}
	if err := ensureVersion(ctx); err != nil {
		return nil, fmt.Errorf("claude auth login: %w", err)
	}
	args := []string{"auth", "login"}
	if email != "" {
		// the value is a separate argument, so one starting with a dash
		// would be read as another flag
		if strings.HasPrefix(email, "-") {
			return nil, errors.New("not a usable email for claude auth login")
		}
		args = append(args, "--email", email)
	}
	env, _ := Env(os.Environ(), configDir)
	cmd, _, err := command(ctx, env, args...)
	return cmd, err
}

// ErrSignOutUnknown means Logout could not tell whether the folder ended up
// signed out: the call was cut short, or its result could not be checked. a
// failure that claude itself reported is not this, and neither is a folder that
// is known to be still signed in.
var ErrSignOutUnknown = errors.New("sign-out state unknown")

// Logout signs configDir out through claude, then asks for the status to check
// it worked: the exit code alone is not proof. a call without a deadline gets
// authTimeout, and callers must not use short ones.
func Logout(ctx context.Context, configDir string) error {
	if err := checkFolder(configDir); err != nil {
		return fmt.Errorf("claude auth logout: %w", err)
	}
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()
	if err := ensureVersion(ctx); err != nil {
		return fmt.Errorf("claude auth logout: %w", err)
	}
	env, _ := Env(os.Environ(), configDir)
	res, err := run(ctx, env, "auth", "logout")
	var cut *CancelError
	switch {
	case errors.As(err, &cut):
		return fmt.Errorf("claude auth logout: %w: %w", ErrSignOutUnknown, err)
	case err != nil:
		return fmt.Errorf("claude auth logout: %w", err)
	case res.signal != "" || res.code != 0:
		return res.failure("auth logout", res.ended())
	}
	st, err := Status(ctx, configDir)
	if err != nil {
		return fmt.Errorf("claude auth logout: cannot confirm it worked: %w: %w", ErrSignOutUnknown, err)
	}
	if st.LoggedIn && st.AuthMethod == "claude.ai" {
		return errors.New("claude auth logout: the folder is still signed in afterwards")
	}
	return nil
}
