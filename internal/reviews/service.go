package reviews

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Interval is how often the watcher polls.
const Interval = 5 * time.Minute

const suggestTTL = 10 * time.Minute

// errNoRoom stops a poll: no account has 5h room, the next poll tries again.
var errNoRoom = errors.New("reviews: no account has room in its 5h window")

// Service is the watcher and the reviews it keeps. Dir holds config.json and
// the results, CacheDir the clones, WorkDir the temporary worktrees.
type Service struct {
	Dir, CacheDir, WorkDir string
	GH                     GH
	Git                    Git
	Claude                 Exec
	ClaudeBin              string
	// EnvFor is the environment of a claude run on an account folder.
	EnvFor func(configDir string) []string
	// Accounts lists the accounts a review may run on, with their 5h figures.
	Accounts func() []Candidate
	Now      func() time.Time
	Logf     func(format string, args ...any)
	// ChunkBytes is the diff budget of one claude run, 0 means chunkBytes.
	ChunkBytes int

	mu          sync.Mutex
	cfg         Config
	loaded      bool
	lastChecked time.Time
	running     *Running
	sugg        []string
	suggErr     string
	suggAt      time.Time
	// tickMu keeps it to one review at a time, a Tick and the next wait.
	tickMu sync.Mutex
	kick   chan struct{}
}

// Running is the review in progress.
type Running struct {
	Repo   string    `json:"repo"`
	Number int       `json:"number"`
	Title  string    `json:"title"`
	Since  time.Time `json:"since"`
}

// Status is what the snapshot and the inbox need.
type Status struct {
	Enabled     bool       `json:"enabled"`
	Running     *Running   `json:"running"`
	LastChecked *time.Time `json:"lastChecked"`
	Count       int        `json:"count"`
}

// SettingsView is what Settings shows.
type SettingsView struct {
	Enabled          bool       `json:"enabled"`
	Repos            []string   `json:"repos"`
	Suggestions      []string   `json:"suggestions"`
	SuggestionsError string     `json:"suggestionsError,omitempty"`
	LastChecked      *time.Time `json:"lastChecked"`
}

func (s *Service) logf(f string, a ...any) {
	if s.Logf != nil {
		s.Logf(f, a...)
	}
}

func (s *Service) store() Store { return Store{Dir: s.Dir} }

func (s *Service) config() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		c, err := LoadConfig(s.Dir)
		if err != nil {
			s.logf("reviews config: %v", err)
		}
		s.cfg, s.loaded = c, true
	}
	return s.cfg
}

// SaveSettings stores the config and wakes the watcher.
func (s *Service) SaveSettings(c Config) error {
	if err := SaveConfig(s.Dir, c); err != nil {
		return err
	}
	c.Repos = cleanRepos(c.Repos)
	s.mu.Lock()
	s.cfg, s.loaded = c, true
	kick := s.kick
	s.mu.Unlock()
	if kick != nil && c.Enabled {
		select {
		case kick <- struct{}{}:
		default:
		}
	}
	return nil
}

// SettingsView is the config plus suggestions from gh repo list (kept for a
// while, a failure is a note, never an error).
func (s *Service) SettingsView(ctx context.Context) SettingsView {
	cfg := s.config()
	s.mu.Lock()
	fresh := !s.suggAt.IsZero() && s.Now().Sub(s.suggAt) < suggestTTL
	s.mu.Unlock()
	if !fresh {
		names, err := s.GH.RepoList(ctx)
		s.mu.Lock()
		s.sugg, s.suggErr, s.suggAt = names, "", s.Now()
		if err != nil {
			s.sugg, s.suggErr = nil, "could not list your repositories with gh"
			s.logf("reviews suggestions: %v", err)
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	opted := map[string]bool{}
	for _, r := range cfg.Repos {
		opted[r] = true
	}
	v := SettingsView{Enabled: cfg.Enabled, Repos: cfg.Repos, Suggestions: []string{}, SuggestionsError: s.suggErr}
	for _, n := range s.sugg {
		if !opted[n] {
			v.Suggestions = append(v.Suggestions, n)
		}
	}
	if !s.lastChecked.IsZero() {
		t := s.lastChecked
		v.LastChecked = &t
	}
	return v
}

// Status reports whether reviews are on, what runs now and how many exist.
func (s *Service) Status() Status {
	cfg := s.config()
	st := Status{Enabled: cfg.Enabled}
	s.mu.Lock()
	if s.running != nil {
		r := *s.running
		st.Running = &r
	}
	if !s.lastChecked.IsZero() {
		t := s.lastChecked
		st.LastChecked = &t
	}
	s.mu.Unlock()
	list, _ := s.store().List()
	st.Count = len(list)
	return st
}

// List is every review, newest first.
func (s *Service) List() ([]Review, error) { return s.store().List() }

// Get is one review by id.
func (s *Service) Get(id string) (Review, bool, error) { return s.store().Get(id) }

// Tick is one poll: for each opted-in repo, find the pull requests that ask
// for your review, and review the ones with no stored result for their head
// commit, one at a time.
func (s *Service) Tick(ctx context.Context) error {
	_, err := s.tick(ctx)
	return err
}

// tick is Tick plus whether it already refreshed the stored pr states, so
// poll can skip a second gh lookup per stored open pr in the same poll.
func (s *Service) tick(ctx context.Context) (refreshed bool, err error) {
	cfg := s.config()
	if !cfg.Enabled || len(cfg.Repos) == 0 {
		return false, nil
	}
	s.tickMu.Lock()
	defer s.tickMu.Unlock()
	defer func() {
		s.mu.Lock()
		s.lastChecked = s.Now()
		s.mu.Unlock()
	}()
	// refresh first so retries see current merged/closed states
	s.refreshStates(ctx)
	refreshed = true
	for _, repo := range cfg.Repos {
		prs, err := s.GH.SearchRequested(ctx, repo)
		if err != nil {
			s.logf("reviews: search %s: %v", repo, err)
			continue
		}
		for _, pr := range prs {
			if ctx.Err() != nil {
				return refreshed, ctx.Err()
			}
			if err := s.handle(ctx, pr); err != nil {
				if errors.Is(err, errNoRoom) {
					return refreshed, err
				}
				s.logf("reviews: %s#%d: %v", pr.Repo, pr.Number, err)
			}
		}
	}
	return refreshed, nil
}

func (s *Service) handle(ctx context.Context, pr PR) error {
	info, err := s.GH.View(ctx, pr.Repo, pr.Number)
	if err != nil {
		return err
	}
	if len(info.SHA) < 7 || s.done(pr.Repo, pr.Number, info.SHA) {
		return nil
	}
	now := s.Now()
	checked := now
	cand, ok := Pick(s.Accounts())
	if !ok {
		return errNoRoom
	}
	s.mu.Lock()
	s.running = &Running{Repo: pr.Repo, Number: pr.Number, Title: tidy(info.Title, maxTitle), Since: now}
	s.mu.Unlock()
	rv, err := s.review(ctx, pr, info, cand)
	s.mu.Lock()
	s.running = nil
	s.mu.Unlock()
	if err != nil {
		return err
	}
	// a run that was cut by the server stopping is not a result
	if ctx.Err() != nil {
		return ctx.Err()
	}
	prev := 0
	if old, ok, gerr := s.store().Get(idOf(pr.Repo, pr.Number, info.SHA)); gerr == nil && ok {
		prev = failedAttempts(old)
	}
	stampFailure(&rv, prev, s.Now())
	rv.State, rv.StateCheckedAt = StateOpen, &checked
	return s.store().Save(rv)
}

// poll is one Run iteration. Tick already refreshes states when it gets past
// the disabled check, so only refresh here when it did not; otherwise every
// stored open pr would get two gh state lookups per poll.
func (s *Service) poll(ctx context.Context) error {
	refreshed, err := s.tick(ctx)
	if !refreshed {
		s.RefreshStates(ctx)
	}
	return err
}

// Run polls until ctx ends: at once, then every Interval, and again when the
// settings are saved with reviews on.
func (s *Service) Run(ctx context.Context) {
	s.mu.Lock()
	s.kick = make(chan struct{}, 1)
	kick := s.kick
	s.mu.Unlock()
	t := time.NewTicker(Interval)
	defer t.Stop()
	for {
		if err := s.poll(ctx); err != nil && ctx.Err() == nil {
			s.logf("reviews: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-kick:
		}
	}
}
