// Package reviews watches the pull requests that ask for your review, runs a
// read-only claude review of each one and keeps the results. it never writes
// to GitHub: gh runs only through the allowlist in this file, and clalt never
// reads or stores a GitHub token.
package reviews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Exec runs a program and returns its stdout. dir is the working folder
// (empty for the current one), env is the whole environment (nil to inherit).
type Exec interface {
	Run(ctx context.Context, dir string, env []string, stdin string, name string, args ...string) ([]byte, error)
}

// ErrNotAllowed is a gh argv outside the allowlist. nothing was run.
var ErrNotAllowed = errors.New("reviews: this gh command is not on the read allowlist")

// ghRule is one allowed command: the flags it may carry, and which of them
// take a value.
type ghRule struct {
	valued map[string]bool
	bare   map[string]bool
}

func rule(valued []string, bare ...string) ghRule {
	r := ghRule{valued: map[string]bool{}, bare: map[string]bool{}}
	for _, v := range valued {
		r.valued[v] = true
	}
	for _, b := range bare {
		r.bare[b] = true
	}
	return r
}

// ghAllowed is every gh command this package may run, by its first two words.
// all of them only read from GitHub (checkout and clone write to local disk).
var ghAllowed = map[string]ghRule{
	"search prs":  rule([]string{"--repo", "--json", "--limit"}, "--review-requested=@me", "--state=open"),
	"pr view":     rule([]string{"--repo", "--json"}),
	"pr diff":     rule([]string{"--repo"}),
	"pr checkout": rule([]string{"--repo"}, "--detach"),
	"repo list":   rule([]string{"--json", "--limit"}, "--no-archived", "--source"),
	"repo clone":  rule(nil),
}

// allowedGH says whether argv is one of the allowed commands with only flags
// that command is known to need. a flag that is not listed is refused, so a
// body, an approval or a method flag can never ride along.
func allowedGH(args []string) error {
	if len(args) < 2 {
		return ErrNotAllowed
	}
	r, ok := ghAllowed[args[0]+" "+args[1]]
	if !ok {
		return ErrNotAllowed
	}
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case r.bare[a]:
		case r.valued[a]:
			i++
			if i >= len(rest) || strings.HasPrefix(rest[i], "-") {
				return ErrNotAllowed
			}
		case strings.HasPrefix(a, "-"):
			return ErrNotAllowed
		}
	}
	return nil
}

// GH is the one way this package runs gh.
type GH struct{ X Exec }

func (g GH) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if err := allowedGH(args); err != nil {
		return nil, fmt.Errorf("%w: gh %s", err, strings.Join(args, " "))
	}
	return g.X.Run(ctx, dir, nil, "", "gh", args...)
}

var repoRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)

// ValidRepo says whether s is owner/name and nothing else.
func ValidRepo(s string) bool {
	return repoRe.MatchString(s) && !strings.HasPrefix(s, ".") && !strings.Contains(s, "..")
}

// PR is a pull request that asks for your review.
type PR struct {
	Repo   string
	Number int
	Title  string
	Author string
	URL    string
}

// PRInfo is what gh pr view says: the head commit and the size.
type PRInfo struct {
	SHA    string
	Lines  int
	Files  int
	Title  string
	URL    string
	Author string
}

type login struct {
	Login string `json:"login"`
}

// SearchRequested lists the open, non-draft pull requests of repo that ask you
// for a review.
func (g GH) SearchRequested(ctx context.Context, repo string) ([]PR, error) {
	if !ValidRepo(repo) {
		return nil, fmt.Errorf("reviews: %q is not owner/name", repo)
	}
	out, err := g.run(ctx, "", "search", "prs", "--review-requested=@me", "--state=open", "--repo", repo,
		"--json", "number,title,url,isDraft,author,repository", "--limit", "30")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		URL        string `json:"url"`
		IsDraft    bool   `json:"isDraft"`
		Author     login  `json:"author"`
		Repository struct {
			Name string `json:"nameWithOwner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("reviews: gh search answered something else: %w", err)
	}
	var prs []PR
	for _, r := range rows {
		if r.IsDraft || r.Number <= 0 {
			continue
		}
		prs = append(prs, PR{Repo: repo, Number: r.Number, Title: r.Title, Author: r.Author.Login, URL: r.URL})
	}
	return prs, nil
}

// View reads the head commit and size of one pull request.
func (g GH) View(ctx context.Context, repo string, number int) (PRInfo, error) {
	out, err := g.run(ctx, "", "pr", "view", strconv.Itoa(number), "--repo", repo,
		"--json", "headRefOid,additions,deletions,changedFiles,title,url,author")
	if err != nil {
		return PRInfo{}, err
	}
	var v struct {
		SHA    string `json:"headRefOid"`
		Add    int    `json:"additions"`
		Del    int    `json:"deletions"`
		Files  int    `json:"changedFiles"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		Author login  `json:"author"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return PRInfo{}, fmt.Errorf("reviews: gh pr view answered something else: %w", err)
	}
	return PRInfo{SHA: v.SHA, Lines: v.Add + v.Del, Files: v.Files, Title: v.Title, URL: v.URL, Author: v.Author.Login}, nil
}

// PRState says whether a pull request is open, merged or closed. any other
// answer, or output that is not the expected json, is an error.
func (g GH) PRState(ctx context.Context, repo string, number int) (string, error) {
	out, err := g.run(ctx, "", "pr", "view", strconv.Itoa(number), "--repo", repo, "--json", "state")
	if err != nil {
		return "", err
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", fmt.Errorf("reviews: gh pr view answered something else: %w", err)
	}
	switch v.State {
	case "OPEN":
		return "open", nil
	case "MERGED":
		return "merged", nil
	case "CLOSED":
		return "closed", nil
	}
	return "", fmt.Errorf("reviews: gh pr view gave an unknown state %q", v.State)
}

// Diff is the unified diff of a pull request.
func (g GH) Diff(ctx context.Context, repo string, number int) (string, error) {
	out, err := g.run(ctx, "", "pr", "diff", strconv.Itoa(number), "--repo", repo)
	return string(out), err
}

// RepoList names the repositories you can see, for the settings suggestions.
func (g GH) RepoList(ctx context.Context) ([]string, error) {
	out, err := g.run(ctx, "", "repo", "list", "--json", "nameWithOwner", "--limit", "100", "--no-archived")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Name string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("reviews: gh repo list answered something else: %w", err)
	}
	var names []string
	for _, r := range rows {
		if ValidRepo(r.Name) {
			names = append(names, r.Name)
		}
	}
	return names, nil
}

// Clone makes the local clone of repo at dest.
func (g GH) Clone(ctx context.Context, repo, dest string) error {
	_, err := g.run(ctx, "", "repo", "clone", repo, dest)
	return err
}

// Checkout puts the pull request in dir, a worktree, without a branch name.
func (g GH) Checkout(ctx context.Context, dir, repo string, number int) error {
	_, err := g.run(ctx, dir, "pr", "checkout", strconv.Itoa(number), "--repo", repo, "--detach")
	return err
}
