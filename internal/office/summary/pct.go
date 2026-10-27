package summary

import "time"

// readings of one account from since on, none when no source is set.
func (s *Service) readings(id string, since time.Time) []Reading {
	if s.cfg.Readings == nil {
		return nil
	}
	return s.cfg.Readings(id, since)
}

func raise(cur **float64, v *float64) {
	if v != nil && (*cur == nil || *v > **cur) {
		x := *v
		*cur = &x
	}
}

// historyPercents sets the peak percents of each day from the readings log:
// the highest reading whose local day it is. days with no reading stay null.
func (s *Service) historyPercents(days []HistoryDay, id string, first, end time.Time) {
	for _, r := range s.readings(id, first) {
		if r.T.Before(first) || !r.T.Before(end) {
			continue
		}
		d := &days[dayIndex(r.T.In(s.cfg.Loc), first, s.cfg.Loc)]
		raise(&d.Peak5hPct, r.FiveHour)
		raise(&d.Peak7dPct, r.SevenDay)
	}
}

// dayPercents sets the peak and the latest 5h percent of an account for the
// window. today with no reading yet takes the live percent instead.
func (s *Service) dayPercents(a *AccountOut, w window) {
	var latest time.Time
	for _, r := range s.readings(a.ID, w.from) {
		if !w.has(r.T) || r.FiveHour == nil {
			continue
		}
		raise(&a.Peak5hPct, r.FiveHour)
		if !r.T.Before(latest) {
			latest = r.T
			v := *r.FiveHour
			a.Last5hPct = &v
		}
	}
	if a.Peak5hPct == nil && s.cfg.Limits != nil && w.has(s.cfg.Now()) {
		if five, _ := s.cfg.Limits(a.ID); five != nil {
			v, l := *five, *five
			a.Peak5hPct, a.Last5hPct = &v, &l
		}
	}
}
