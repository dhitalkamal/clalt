package summary

import (
	"errors"
	"sync"
	"time"

	"github.com/dhitalkamal/clalt/internal/usage"
)

// ErrBadDay is a day that is not YYYY-MM-DD.
var ErrBadDay = errors.New("summary: day must be YYYY-MM-DD")

// ErrBadDays is a history length outside 1..MaxDays.
var ErrBadDays = errors.New("summary: days must be from 1 to 90")

// history lengths.
const (
	MaxDays     = 90
	DefaultDays = 30
	cacheTTL    = 30 * time.Second
	maxCached   = 32
)

// Account is an account as the page labels it.
type Account struct {
	ID    string
	Label string
}

// Config is what the service reads. Roots are the folders that hold the
// project folders of transcripts. Attribute places records on accounts, as
// internal/usage does. Limits gives the live 5h and 7d percent of an account,
// nil when unknown. Now and Loc are the clock and the zone that days are cut
// in, time.Now and time.Local when unset. zero MaxFiles or MaxBytes mean the
// defaults.
type Config struct {
	Roots     []string
	Accounts  func() ([]Account, error)
	Attribute func([]usage.Record) (map[string][]usage.Record, error)
	Limits    func(accountID string) (five, seven *float64)
	Now       func() time.Time
	Loc       *time.Location
	MaxFiles  int
	MaxBytes  int64
}

// Service builds the two replies and caches each for 30 seconds.
type Service struct {
	cfg   Config
	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	at  time.Time
	val any
}

// New fills in the defaults of cfg.
func New(cfg Config) *Service {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Loc == nil {
		cfg.Loc = time.Local
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = defaultMaxFiles
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultMaxBytes
	}
	return &Service{cfg: cfg, cache: map[string]cached{}}
}

func (s *Service) dayStart(t time.Time) time.Time {
	y, m, d := t.In(s.cfg.Loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, s.cfg.Loc)
}

// memo returns the cached value of key while it is fresh, else builds it. a
// build holds the lock, so two requests for one day read the files once.
func (s *Service) memo(key string, build func() (any, error)) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.cfg.Now()
	if c, ok := s.cache[key]; ok && now.Sub(c.at) < cacheTTL && !now.Before(c.at) {
		return c.val, nil
	}
	v, err := build()
	if err != nil {
		return nil, err
	}
	for k, c := range s.cache {
		if now.Sub(c.at) >= cacheTTL {
			delete(s.cache, k)
		}
	}
	if len(s.cache) >= maxCached {
		clear(s.cache)
	}
	s.cache[key] = cached{at: now, val: v}
	return v, nil
}

// Day is the summary of a local day, today when day is empty.
func (s *Service) Day(day string) (Summary, error) {
	start := s.dayStart(s.cfg.Now())
	if day != "" {
		t, err := time.ParseInLocation("2006-01-02", day, s.cfg.Loc)
		if err != nil || t.Format("2006-01-02") != day {
			return Summary{}, ErrBadDay
		}
		start = t
	}
	v, err := s.memo("day:"+start.Format("2006-01-02"), func() (any, error) { return s.buildDay(start) })
	if err != nil {
		return Summary{}, err
	}
	return v.(Summary), nil
}

// History is the last days days ending today, 0 meaning the default.
func (s *Service) History(days int) (History, error) {
	if days == 0 {
		days = DefaultDays
	}
	if days < 1 || days > MaxDays {
		return History{}, ErrBadDays
	}
	today := s.dayStart(s.cfg.Now())
	v, err := s.memo("history:"+today.Format("2006-01-02")+":"+itoa(days), func() (any, error) { return s.buildHistory(today, days) })
	if err != nil {
		return History{}, err
	}
	return v.(History), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func (s *Service) accounts() ([]Account, error) {
	if s.cfg.Accounts == nil {
		return nil, nil
	}
	return s.cfg.Accounts()
}

func (s *Service) attribute(recs []usage.Record) (map[string][]usage.Record, error) {
	if s.cfg.Attribute == nil {
		return map[string][]usage.Record{usage.Unattributed: recs}, nil
	}
	return s.cfg.Attribute(recs)
}

func (s *Service) buildDay(start time.Time) (Summary, error) {
	w := window{from: start, to: start.AddDate(0, 0, 1)}
	sc, truncated := readAll(candidates(s.cfg.Roots, w.from), s.cfg.MaxFiles, s.cfg.MaxBytes, w)
	order, err := s.accounts()
	if err != nil {
		return Summary{}, err
	}
	sc.records = usage.Dedup(sc.records)
	by, err := s.attribute(sc.records)
	if err != nil {
		return Summary{}, err
	}
	sum := buildDay(sc, by, w, order)
	sum.Truncated = truncated
	return sum, nil
}
