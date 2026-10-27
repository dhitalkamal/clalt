package cli

import "github.com/dhitalkamal/clalt/internal/feedinstall"

// linkNotice says, before the question, which file will really change when
// settings.json is a link, and what an edit in a git work tree means.
func (a *App) linkNotice(t feedinstall.Target, settings string) {
	if !t.Link {
		return
	}
	a.printf("%s is a link: clalt edits the real file %s and leaves the link as it is.\n", clean(settings), clean(t.Path))
	if t.GitRoot != "" {
		a.printf("that file is in the git work tree %s, so the change shows up in that repo. the hooks use this machine's clalt path and are silent no-ops on machines where that path does not exist.\n", clean(t.GitRoot))
	}
}
