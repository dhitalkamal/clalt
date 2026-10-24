package garden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/office/layout"
)

// TestWriteDebug writes generated layouts as json and svg into the directory
// named by GARDEN_DEBUG_DIR, for looking at them. it does nothing otherwise.
func TestWriteDebug(t *testing.T) {
	dir := os.Getenv("GARDEN_DEBUG_DIR")
	if dir == "" {
		t.Skip("GARDEN_DEBUG_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tpl := range templates {
		for _, n := range []int{0, 3, 8, 12} {
			l, err := Generate(tpl, inputFor(n, [3]int{2, 8, 4}))
			if n == 3 {
				l, err = Generate(tpl, three())
			}
			if err != nil {
				t.Log(err)
				continue
			}
			b, err := layout.Encode(l)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(dir, fmt.Sprintf("%s-%d", tpl, n))
			if err := os.WriteFile(base+".json", b, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(base+".svg", []byte(svgOf(l)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// svgOf draws a layout flat, top down, 10 px per unit.
func svgOf(l layout.Layout) string {
	var b strings.Builder
	lox, loz, hix, hiz := 1e9, 1e9, -1e9, -1e9
	for _, p := range l.Site.Base.Poly {
		lox, hix = min(lox, p[0]), max(hix, p[0])
		loz, hiz = min(loz, p[1]), max(hiz, p[1])
	}
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%.1f %.1f %.1f %.1f" width="%d" height="%d"><rect x="-300" y="-300" width="600" height="600" fill="#bfe3f5"/>`,
		lox-1, loz-1, hix-lox+2, hiz-loz+2, int((hix-lox+2)*10), int((hiz-loz+2)*10))
	poly := func(ps []pt, fill string) {
		b.WriteString(`<polygon fill="` + fill + `" points="`)
		for _, p := range ps {
			fmt.Fprintf(&b, "%.2f,%.2f ", p[0], p[1])
		}
		b.WriteString(`"/>`)
	}
	poly(l.Site.Base.Poly, "#9bd37a")
	line := func(ps []pt, w float64, c string) {
		b.WriteString(`<polyline fill="none" stroke-linejoin="round" stroke-linecap="round" stroke="` + c + fmt.Sprintf(`" stroke-width="%.2f" points="`, w))
		for _, p := range ps {
			fmt.Fprintf(&b, "%.2f,%.2f ", p[0], p[1])
		}
		b.WriteString(`"/>`)
	}
	if l.Site.River != nil {
		line(l.Site.River.Pts, l.Site.River.W, "#6cb8e6")
	}
	for _, p := range l.Site.Ponds {
		fmt.Fprintf(&b, `<ellipse cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" fill="#6cb8e6"/>`, p.X, p.Z, p.RX, p.RZ)
	}
	for _, p := range l.Site.Paths {
		line(p.Pts, p.W, "#e3d9c6")
	}
	for _, br := range l.Site.Bridges {
		line([]pt{br.A, br.B}, br.W, "#a8743c")
	}
	colors := map[string]string{"lobby": "#999", "cabin": "#e8a", "research": "#8ae", "workspace": "#ea8", "treehouse": "#a8e", "cafe": "#fc6", "play": "#f9a"}
	for _, p := range l.Pavilions {
		poly(corners(p), colors[p.Type])
		d := door(p)
		fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="0.25" fill="red"/><text x="%.2f" y="%.2f" font-size="0.9" text-anchor="middle">%s</text>`, d[0], d[1], p.X, p.Z, p.ID)
	}
	for _, p := range l.Site.Props {
		fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="0.3" fill="#444"/>`, p.X, p.Z)
	}
	for _, p := range l.Overflow {
		fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="0.3" fill="#27a"/>`, p.X, p.Z)
	}
	b.WriteString(`</svg>`)
	return b.String()
}
