package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/conversation"
)

// resumeConversation continues one conversation under an account, through the
// same path as `clalt run <name> -- --resume <id>`, from the folder it ran in.
func (a *App) resumeConversation(root, word, label string) error {
	c, err := conversation.Find(root, word)
	var amb *conversation.AmbiguousError
	switch {
	case errors.Is(err, conversation.ErrNotFound):
		return fmt.Errorf("no conversation starts with %q. 'clalt resume' lists them", clean(word))
	case errors.As(err, &amb):
		return ambiguousError(amb)
	case err != nil:
		return err
	}

	acc, err := a.resumeAccount(c, label)
	if err != nil {
		return err
	}
	name := shown(acc)
	if err := checkFolderThere(c, name); err != nil {
		return err
	}
	running, err := a.deps().sessions()
	if err != nil {
		return fmt.Errorf("could not read the running sessions: %w", err)
	}
	for _, s := range running {
		if s.ClaudeSessionID != "" && strings.EqualFold(s.ClaudeSessionID, c.ID) {
			return fmt.Errorf("that conversation is open right now, in session %s. 'clalt jump %s' goes to it. to open it a second time anyway, which mixes both into one transcript, run: clalt run %s -- --resume %s",
				clean(s.Handle), clean(s.Handle), name, c.ID)
		}
	}

	fmt.Fprintf(a.Err, "resuming %s as %s in %s\n", c.ID[:8], name, clean(c.Cwd))
	return runIn(c.Cwd, func() error {
		return a.cmdRun([]string{acc.Label(), "--", "--resume", c.ID})
	})
}

func ambiguousError(amb *conversation.AmbiguousError) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%d conversations start with %q, give more of the id:", len(amb.Matches), clean(amb.Prefix))
	for _, m := range amb.Matches {
		fmt.Fprintf(&b, "\n  %s  %s", m.ID, clean(shorten(m.Title, 60)))
	}
	return errors.New(b.String())
}

// resumeAccount is the account named by --account, else the one that last ran
// the conversation when the feed knows it and it still exists, else the user is
// asked to name one.
func (a *App) resumeAccount(c conversation.Conversation, label string) (account.Account, error) {
	if label != "" {
		return a.findVisible(label)
	}
	all, err := a.Store.List()
	if err != nil {
		return account.Account{}, err
	}
	if id, ok := a.deps().lastAccount(c.ID); ok {
		for _, acc := range all {
			if acc.Name == id && !acc.Meta.Hidden {
				return acc, nil
			}
		}
	}
	var names []string
	for _, acc := range all {
		if !acc.Meta.Hidden {
			names = append(names, shown(acc))
		}
	}
	msg := "which account? no account is known for this conversation, pass --account <name>"
	if len(names) > 0 {
		msg += " (" + strings.Join(names, ", ") + ")"
	}
	return account.Account{}, usageError{msg, resumeUsage}
}

// checkFolderThere refuses to run a conversation from somewhere else when the
// folder it ran in is gone: claude would resume it in the wrong project.
func checkFolderThere(c conversation.Conversation, name string) error {
	hatch := fmt.Sprintf("to resume it from the folder you are in, run: clalt run %s -- --resume %s", name, c.ID)
	if c.Cwd == "" {
		return fmt.Errorf("the transcript does not say which folder this conversation ran in. %s", hatch)
	}
	if !filepath.IsAbs(c.Cwd) {
		return fmt.Errorf("the folder recorded for this conversation, %s, is not an absolute path. %s", clean(c.Cwd), hatch)
	}
	fi, err := os.Stat(c.Cwd)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("the folder this conversation ran in no longer exists: %s. %s", clean(c.Cwd), hatch)
	}
	return nil
}

// runIn runs fn with dir as the working folder and as PWD, which claude's
// children read, and puts both back afterwards.
func runIn(dir string, fn func() error) error {
	prev, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	oldPWD, hadPWD := os.LookupEnv("PWD")
	os.Setenv("PWD", dir)
	defer func() {
		_ = os.Chdir(prev)
		if hadPWD {
			os.Setenv("PWD", oldPWD)
		} else {
			os.Unsetenv("PWD")
		}
	}()
	return fn()
}
