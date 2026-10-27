package account

import "slices"

// Rules is what an account may serve and whether auto-switch may use it. no
// folders means any folder. what a folder rule matches is for package rules to
// say: the store only keeps the strings.
type Rules struct {
	Folders    []string `json:"folders,omitempty"`
	AutoSwitch bool     `json:"autoSwitch,omitempty"`
}

func (r Rules) clone() Rules {
	return Rules{Folders: slices.Clone(r.Folders), AutoSwitch: r.AutoSwitch}
}

// RulesOf gives the rules of an account, none when it has none.
func (s *Store) RulesOf(id string) (Rules, error) {
	st, err := s.load()
	if err != nil {
		return Rules{}, err
	}
	if _, ok := st.Accounts[id]; !ok {
		return Rules{}, errNoAccount(id)
	}
	return st.Rules[id].clone(), nil
}

// AllRules gives the rules of every account that has some, by id, in one read of
// the file.
func (s *Store) AllRules() (map[string]Rules, error) {
	st, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make(map[string]Rules, len(st.Rules))
	for id, r := range st.Rules {
		out[id] = r.clone()
	}
	return out, nil
}

// UpdateRules changes the rules of an account in one load-modify-save cycle
// under the state lock, so two commands that edit rules at once cannot lose each
// other's change. fn gets a copy and returns the rules to keep, and rules with no
// folder and no auto-switch are not kept at all, so the file goes back to what it
// was. its error is returned and nothing is written. like every mutator it must
// not call back into the store.
func (s *Store) UpdateRules(id string, fn func(Rules) (Rules, error)) error {
	return s.mutate(func(st *state) error {
		if _, ok := st.Accounts[id]; !ok {
			return errNoAccount(id)
		}
		next, err := fn(st.Rules[id].clone())
		if err != nil {
			return err
		}
		if len(next.Folders) == 0 && !next.AutoSwitch {
			delete(st.Rules, id)
			return nil
		}
		if st.Rules == nil {
			st.Rules = map[string]Rules{}
		}
		st.Rules[id] = next.clone()
		return nil
	})
}
