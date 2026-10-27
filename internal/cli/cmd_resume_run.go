package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/conversation"
	"github.com/dhitalkamal/clalt/internal/rules"
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

	acc, note, err := a.resumeAccount(c, label)
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

	if note != "" {
		fmt.Fprintln(a.Err, note)
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
// the conversation when the log knows it, it still exists and its folder rules
// allow the conversation's folder, else the picker's choice for that folder.
// note is a line for stderr to print once the checks that follow have passed.
func (a *App) resumeAccount(c conversation.Conversation, label string) (acc account.Account, note string, err error) {
	if label != "" {
		acc, err = a.findVisible(label)
		return acc, "", err
	}
	all, err := a.Store.List()
	if err != nil {
		return account.Account{}, "", err
	}
	id, ok := a.deps().lastAccount(c.ID)
	for _, cand := range all {
		if !ok || cand.Name != id || cand.Meta.Hidden {
			continue
		}
		allRules, err := a.Store.AllRules()
		if err != nil {
			return account.Account{}, "", err
		}
		if c.Cwd == "" || rules.Allows(allRules[cand.Name].Folders, c.Cwd) {
			return cand, "", nil
		}
		fmt.Fprintf(a.Err, "%s last ran this conversation, but its folder rules do not include %s\n", clean(cand.Label()), clean(c.Cwd))
	}
	return a.pickForResume(c.Cwd)
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
