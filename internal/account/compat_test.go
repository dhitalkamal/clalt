package account

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// v010State is the shape the v0.1.0 binary reads and writes.
type v010State struct {
	Active   string `json:"active"`
	Accounts map[string]struct {
		Email        string    `json:"email,omitempty"`
		Plan         string    `json:"plan,omitempty"`
		AddedAt      time.Time `json:"addedAt"`
		LastUsedAt   time.Time `json:"lastUsedAt,omitempty"`
		DailyBudget  float64   `json:"dailyBudget,omitempty"`
		WeeklyBudget float64   `json:"weeklyBudget,omitempty"`
	} `json:"accounts"`
}

const v010File = `{
  "active": "work",
  "accounts": {
    "work": {
      "email": "w@example.com",
      "plan": "max",
      "addedAt": "2026-08-01T10:00:00Z",
      "lastUsedAt": "2026-10-01T09:30:00Z",
      "dailyBudget": 25,
      "weeklyBudget": 100
    }
  }
}`

func TestStateWrittenByThisVersionIsReadableByV010(t *testing.T) {
	s, _ := newStore(t)
	acc, err := s.Create("robin", noFolders)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetIdentity(acc.Name, Identity{Email: "k@example.com", Org: "Example Org", Plan: "max"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	var topKeys []string
	for k := range top {
		topKeys = append(topKeys, k)
	}
	sort.Strings(topKeys)
	if !reflect.DeepEqual(topKeys, []string{"accounts", "active"}) {
		t.Errorf("top-level keys = %v, want the v0.1.0 shape", topKeys)
	}
	var old v010State
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatalf("v0.1.0 cannot read the file: %v", err)
	}
	got := old.Accounts["robin"]
	if got.Email != "k@example.com" || got.Plan != "max" || got.AddedAt.IsZero() {
		t.Errorf("v0.1.0 sees %+v", got)
	}
}

func TestStateWrittenByV010LoadsAndSurvivesNewWrites(t *testing.T) {
	s, _ := newStore(t)
	if err := os.WriteFile(s.statePath, []byte(v010File), 0o600); err != nil {
		t.Fatal(err)
	}

	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v, %v", list, err)
	}
	a := list[0]
	if a.Name != "work" || a.Label() != "work" || !a.Active || a.Meta.DailyBudget != 25 || a.Meta.Email != "w@example.com" {
		t.Errorf("account = %+v", a)
	}

	if err := s.Touch("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("fresh", noFolders); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}
	var old v010State
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	work := old.Accounts["work"]
	if old.Active != "work" || work.Email != "w@example.com" || work.Plan != "max" ||
		work.DailyBudget != 25 || work.WeeklyBudget != 100 ||
		!work.AddedAt.Equal(time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("old fields changed: active=%q %+v", old.Active, work)
	}
	if len(old.Accounts) != 2 {
		t.Errorf("accounts = %d, want 2", len(old.Accounts))
	}
	// the file is still in the place the old binary looks
	if filepath.Base(s.statePath) != "state.json" {
		t.Errorf("state path = %s", s.statePath)
	}
}
