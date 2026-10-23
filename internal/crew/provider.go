package crew

import "context"

// Gateway is one gateway as the dashboard lists it.
type Gateway struct {
	Account string
	Port    int
	State   State
}

// Gateways lists the gateways clalt knows, for the dashboard.
func (m *Manager) Gateways(ctx context.Context) ([]Gateway, error) {
	all, err := m.StatusAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Gateway, len(all))
	for i, st := range all {
		out[i] = Gateway{Account: st.Account, Port: st.Record.Port, State: st.State}
	}
	return out, nil
}

// Gateways lists the gateways clalt knows with the real runner and clock. it
// runs nothing when there is no record.
func Gateways(ctx context.Context) ([]Gateway, error) {
	return NewManager(Options{}).Gateways(ctx)
}
