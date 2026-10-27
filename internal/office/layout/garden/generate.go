// Package garden generates the v2 garden campus layouts from the account
// list and the desk needs.
package garden

import (
	"fmt"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// Generate builds the layout of one template: rows, village, river or street.
func Generate(template string, in Input) (layout.Layout, error) {
	p, err := normalize(in)
	if err != nil {
		return layout.Layout{}, err
	}
	switch template {
	case "rows":
		return genRows(p), nil
	case "river":
		return genRiver(p)
	case "street":
		return genStreet(p)
	case "village":
		return genVillage(p)
	}
	return layout.Layout{}, fmt.Errorf("garden: unknown template %q", template)
}
