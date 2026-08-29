package account

import "fmt"

// Fallback returns the rotation chain and threshold. When no chain is configured
// it returns all accounts sorted by name and the default threshold, so `next`
// works out of the box.
func (s *Store) Fallback() ([]string, float64, error) {
	st, err := s.load()
	if err != nil {
		return nil, 0, err
	}
	threshold := st.Threshold
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	chain := st.Fallback
	if len(chain) == 0 {
		for _, ac := range sortedNames(st.Accounts) {
			chain = append(chain, ac)
		}
	}
	return chain, threshold, nil
}

// SetFallback replaces the rotation chain. Every name must be a known account.
func (s *Store) SetFallback(names []string) error {
	st, err := s.load()
	if err != nil {
		return err
	}
	for _, n := range names {
		if _, ok := st.Accounts[n]; !ok {
			return fmt.Errorf("account %q not found", n)
		}
	}
	st.Fallback = names
	return s.save(st)
}

// SetThreshold sets the utilization fraction (0..1) that triggers rotation.
func (s *Store) SetThreshold(t float64) error {
	if t <= 0 || t > 1 {
		return fmt.Errorf("threshold must be between 0 and 1, got %v", t)
	}
	st, err := s.load()
	if err != nil {
		return err
	}
	st.Threshold = t
	return s.save(st)
}

// ClearFallback removes the configured chain (reverting to all-accounts order).
func (s *Store) ClearFallback() error {
	st, err := s.load()
	if err != nil {
		return err
	}
	st.Fallback = nil
	return s.save(st)
}

func sortedNames(m map[string]Meta) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	// simple insertion sort keeps this dependency-free and lists are tiny
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}
