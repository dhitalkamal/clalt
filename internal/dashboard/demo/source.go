package demo

import (
	"context"
	"time"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

// Epoch is the demo's now: the approved frames were taken at 23:53 on
// Wednesday 15 October 2025.
var Epoch = time.Date(2025, time.October, 15, 23, 53, 0, 0, time.UTC)

// Clock stands still at Epoch. the demo runs on it so its numbers never drift.
func Clock() func() time.Time { return func() time.Time { return Epoch } }

type source struct{ now func() time.Time }

var _ dashboard.Source = source{}

// Source is the demo data, built fresh from the ledger on every load. every
// time in it is counted from now, so the demo reads the same whenever it
// runs. the dashboard needs the same clock to show them against.
func Source(now func() time.Time) dashboard.Source { return source{now: now} }

func (s source) Load(ctx context.Context) (dashboard.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return dashboard.Snapshot{}, err
	}
	now := s.now()
	snap := dashboard.Snapshot{Accounts: make([]dashboard.Account, 0, len(people))}
	for _, p := range people {
		snap.Accounts = append(snap.Accounts, p.account(now))
	}
	snap.Removed = []dashboard.Removed{{
		ID: "old-work", Label: "old-work", Email: "old-work@example.com",
		Folder: "~/.config/clalt/profiles/old-work",
	}}
	return snap, nil
}
