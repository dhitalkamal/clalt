package live

import (
	"context"
	"fmt"

	"github.com/dhitalkamal/clalt/internal/crew"
)

// CrewGateways is Gateways on the records of internal/crew. List is
// crew.Gateways when nil.
type CrewGateways struct {
	List func(ctx context.Context) ([]crew.Gateway, error)
}

var _ Gateways = CrewGateways{}

// Running lists the gateways crew has checked are still the process it started.
// the others (stale, unverified, broken) are not running as far as the
// dashboard may say. a gateway listens on loopback only.
func (g CrewGateways) Running(ctx context.Context) ([]Gateway, error) {
	list := g.List
	if list == nil {
		list = crew.Gateways
	}
	all, err := list(ctx)
	if err != nil {
		return nil, err
	}
	var out []Gateway
	for _, gw := range all {
		if gw.State == crew.StateRunning {
			out = append(out, Gateway{AccountID: gw.Account, Addr: fmt.Sprintf("127.0.0.1:%d", gw.Port)})
		}
	}
	return out, nil
}
