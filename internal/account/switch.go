package account

import "fmt"

// Switch makes name the active account. The order is critical: it snapshots the
// currently live credential back into the outgoing slot BEFORE loading the target,
// so a refresh token that Claude Code rotated while the outgoing account was live
// is preserved instead of lost.
func (s *Store) Switch(name string) error {
	// 1. verify the target exists first, so a bad name is a clean no-op
	target, _, err := s.Get(name)
	if err != nil {
		return err
	}

	// 2. write the live credential back to the outgoing slot (captures rotation)
	active, err := s.Active()
	if err != nil {
		return err
	}
	if active != "" && active != name {
		if liveCred, rerr := s.live.Read(); rerr == nil && liveCred.AccessToken != "" {
			_, meta, gerr := s.Get(active)
			if gerr == nil {
				meta.LastUsedAt = s.now()
				if serr := s.Save(active, liveCred, meta); serr != nil {
					return fmt.Errorf("write back active account %q: %w", active, serr)
				}
			}
		}
	}

	// 3. load the target into Claude Code's live store
	if err := s.live.Write(target); err != nil {
		return fmt.Errorf("load %q into live store: %w", name, err)
	}

	// 4. update the active pointer and target's lastUsedAt
	_, tmeta, _ := s.Get(name)
	tmeta.LastUsedAt = s.now()
	if err := s.Save(name, target, tmeta); err != nil {
		return err
	}
	if err := s.SetActive(name); err != nil {
		return err
	}

	// 5. record the switch for usage attribution (best-effort)
	return s.appendSwitchLog(name)
}
