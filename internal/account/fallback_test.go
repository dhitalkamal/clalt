package account

import "testing"

func TestFallbackDefaultsToAllAccounts(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("b", mkCred("b", 1), Meta{})
	_ = s.Save("a", mkCred("a", 1), Meta{})

	chain, threshold, err := s.Fallback()
	if err != nil {
		t.Fatalf("Fallback: %v", err)
	}
	// no chain configured -> all accounts sorted, default threshold
	if len(chain) != 2 || chain[0] != "a" || chain[1] != "b" {
		t.Errorf("chain = %v, want [a b]", chain)
	}
	if threshold != DefaultThreshold {
		t.Errorf("threshold = %v, want default", threshold)
	}
}

func TestSetFallbackAndThreshold(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("a", mkCred("a", 1), Meta{})
	_ = s.Save("b", mkCred("b", 1), Meta{})

	if err := s.SetFallback([]string{"b", "a"}); err != nil {
		t.Fatalf("SetFallback: %v", err)
	}
	if err := s.SetThreshold(0.75); err != nil {
		t.Fatalf("SetThreshold: %v", err)
	}
	chain, threshold, _ := s.Fallback()
	if len(chain) != 2 || chain[0] != "b" {
		t.Errorf("chain = %v, want [b a]", chain)
	}
	if threshold != 0.75 {
		t.Errorf("threshold = %v, want 0.75", threshold)
	}
}

func TestSetFallbackRejectsUnknown(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("a", mkCred("a", 1), Meta{})
	if err := s.SetFallback([]string{"a", "ghost"}); err == nil {
		t.Error("expected error for unknown account in chain")
	}
}

func TestSetThresholdBounds(t *testing.T) {
	s, _ := newStore(t)
	if err := s.SetThreshold(0); err == nil {
		t.Error("0 threshold should error")
	}
	if err := s.SetThreshold(1.5); err == nil {
		t.Error(">1 threshold should error")
	}
}

func TestSetBudget(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("a", mkCred("a", 1), Meta{})
	if err := s.SetBudget("a", 20, 100); err != nil {
		t.Fatalf("SetBudget: %v", err)
	}
	_, meta, _ := s.Get("a")
	if meta.DailyBudget != 20 || meta.WeeklyBudget != 100 {
		t.Errorf("budgets = %v / %v, want 20 / 100", meta.DailyBudget, meta.WeeklyBudget)
	}
	// negative leaves unchanged, zero clears
	_ = s.SetBudget("a", -1, 0)
	_, meta, _ = s.Get("a")
	if meta.DailyBudget != 20 || meta.WeeklyBudget != 0 {
		t.Errorf("after partial update = %v / %v, want 20 / 0", meta.DailyBudget, meta.WeeklyBudget)
	}
	if err := s.SetBudget("ghost", 1, 1); err == nil {
		t.Error("expected error for unknown account")
	}
}

func TestClearFallback(t *testing.T) {
	s, _ := newStore(t)
	_ = s.Save("a", mkCred("a", 1), Meta{})
	_ = s.Save("b", mkCred("b", 1), Meta{})
	_ = s.SetFallback([]string{"b"})
	if err := s.ClearFallback(); err != nil {
		t.Fatalf("ClearFallback: %v", err)
	}
	chain, _, _ := s.Fallback()
	if len(chain) != 2 {
		t.Errorf("after clear chain = %v, want all accounts", chain)
	}
}
