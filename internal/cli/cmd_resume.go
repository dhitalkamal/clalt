package cli

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/conversation"
)

const resumeUsage = "clalt resume [words] [--limit n]   or   clalt resume <id> [--account <name>]"

// defaultResumeLimit is how many conversations the list shows unless asked.
const defaultResumeLimit = 20

// idShaped is what a conversation id, or at least as much of it as the list
// shows, looks like. anything else given to resume is a filter.
var idShaped = regexp.MustCompile(`^[0-9a-fA-F-]{8,}$`)

type resumeArgs struct {
	words    []string
	account  string
	limit    int
	hasLimit bool
}

func parseResumeArgs(args []string) (resumeArgs, error) {
	r := resumeArgs{limit: defaultResumeLimit}
	value := func(i *int, name, inline string, hasInline bool) (string, error) {
		if hasInline {
			return inline, nil
		}
		if *i+1 >= len(args) {
			return "", usageError{name + " needs a value", resumeUsage}
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		name, inline, hasInline := strings.Cut(args[i], "=")
		switch {
		case name == "--account":
			v, err := value(&i, name, inline, hasInline)
			if err != nil {
				return r, err
			}
			r.account = v
		case name == "--limit":
			v, err := value(&i, name, inline, hasInline)
			if err != nil {
				return r, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return r, usageError{"--limit takes a number, 0 for all", resumeUsage}
			}
			r.limit, r.hasLimit = n, true
		case strings.HasPrefix(args[i], "-"):
			return r, usageError{"unknown flag " + args[i], resumeUsage}
		default:
			r.words = append(r.words, args[i])
		}
	}
	return r, nil
}

// cmdResume lists the conversations of every account, or continues one.
func (a *App) cmdResume(args []string) error {
	r, err := parseResumeArgs(args)
	if err != nil {
		return err
	}
	root := config.ClaudeProjectsDir()
	if root == "" {
		return fmt.Errorf("cannot find the claude home: HOME is not set")
	}
	if len(r.words) == 1 && idShaped.MatchString(r.words[0]) {
		if r.hasLimit {
			return usageError{"--limit goes with the list, not with an id", resumeUsage}
		}
		return a.resumeConversation(root, r.words[0], r.account)
	}
	if r.account != "" {
		return usageError{"--account goes with a conversation id", resumeUsage}
	}
	return a.listConversations(root, r)
}

// convRow is a conversation in the json output.
type convRow struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Cwd        string    `json:"cwd"`
	Model      string    `json:"model"`
	LastActive time.Time `json:"lastActive"`
	Size       int64     `json:"size"`
	Project    string    `json:"project"`
	Entrypoint string    `json:"entrypoint"`
}

func (a *App) listConversations(root string, r resumeArgs) error {
	found, err := conversation.List(root, conversation.Options{Words: r.words, Limit: r.limit})
	if err != nil {
		return err
	}
	if a.JSON {
		rows := make([]convRow, 0, len(found))
		for _, c := range found {
			rows = append(rows, convRow{c.ID, c.Title, c.Cwd, c.Model, c.LastActive.UTC(), c.Size, c.Project, c.Entrypoint})
		}
		return a.emitJSON(rows)
	}
	if len(found) == 0 {
		a.printf("no conversations found.\n")
		return nil
	}
	now := a.clock()
	home := os.Getenv("HOME")
	tw := tabwriter.NewWriter(a.out(), 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tLAST ACTIVE\tMODEL\tSIZE\tFOLDER\tTITLE")
	for _, c := range found {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", c.ID[:8], agoText(now, c.LastActive),
			dash(clean(strings.TrimPrefix(c.Model, "claude-"))), sizeText(c.Size),
			dash(clean(shortFolder(c.Cwd, home))), dash(clean(shorten(c.Title, 60))))
	}
	return tw.Flush()
}

func agoText(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Local().Format("2006-01-02")
}

func sizeText(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%dK", n>>10)
	}
	return fmt.Sprintf("%dB", n)
}

// shortFolder shows the home folder as ~ and keeps the end of a long path.
func shortFolder(cwd, home string) string {
	if home != "" && (cwd == home || strings.HasPrefix(cwd, home+"/")) {
		cwd = "~" + strings.TrimPrefix(cwd, home)
	}
	if utf8.RuneCountInString(cwd) > 32 {
		r := []rune(cwd)
		cwd = "..." + string(r[len(r)-29:])
	}
	return cwd
}

func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-3]) + "..."
}
