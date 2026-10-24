# garden campus office

the office default changes from the hall to a garden campus: pavilions on a lawn
island with paths, water and trees. four garden templates are switchable (rows,
village, river, street) and the old hall stays as a fifth (hall). the garden
templates grow with the number of accounts and sessions.

approved look and layouts: the throwaway viewer in the session scratchpad
(office-designs/garden-layouts), layouts 1 to 4. terraces and archipelago were
dropped by the user.

## decisions

- layout format v2 holds the garden. v1 files keep loading and draw as the hall.
- garden templates are generators: go builds the v2 layout from the account
  list and seat needs. a custom v2 file (saved from build mode later) is static.
- growth: a new or removed account changes the layout at once. desk needs grow
  only after holding 30 s and shrink only after holding 10 min, so short
  subagent spikes never rebuild the campus. a spike shorter than that uses
  overflow spots (benches and standing tables the generator places near the
  workspace, cafe and play area), then the lobby, as the routing does today.
- the hall stays as template hall (the current default.json, unchanged).
- switching templates in watch mode is a read: GET /api/layout?template=<id>
  answers the generated layout and the page remembers the pick in the url hash
  and localStorage. making a template the saved default stays an --edit write
  (POST /api/layout/use) with the existing guards. `clalt office --template <id>`
  picks the starting template.
- camera presets on the page: top (75 degrees, default), plan (straight down),
  iso, hero (low 3/4), eye (eye level near the cafe), orbit (free, auto-spin).
- build mode editing of v2 site features (paths, water, pavilion moves) is not
  in this change. build mode keeps editing v1 hall layouts. later work.
- no feature flag: the project has no flag system and the hall stays one click
  away as a template.

## layout v2 contract

world units (one unit is the size used by the viewer, a desk is about 1.0 wide),
x to the east, z to the south, rot in degrees counterclockwise seen from above,
0 faces +z (south, toward the default camera). every number is finite and
inside +-200. points are [x, z] pairs.

```
{
  "v": 2,
  "name": "river",                      // [a-z0-9-]{1,40}
  "title": "River campus",              // <= 60 chars, cleaned
  "template": "rows|village|river|street" or omitted (static custom layout),
  "site": {
    "base": {"poly": [[x,z], ...]},     // island plinth outline, 3..512 points
    "land": [{"poly": [[x,z], ...]}],   // grass areas on the plinth, 1..16 polys
    "river": {"pts": [[x,z], ...], "w": 3.0} or null,   // centre line, 2..256 pts, w 0.5..8
    "ponds": [{"x": 0, "z": 0, "rx": 2, "rz": 1}],      // <= 8, radii 0.5..10
    "paths": [{"pts": [[x,z], ...], "w": 1.0, "smooth": true}],  // <= 160, 2..256 pts, w 0.3..4
    "bridges": [{"a": [x,z], "b": [x,z], "w": 1.3}],    // <= 16, length 1..12
    "props": [{"t": "bench|bed|lamp|fountain|standing-table|sign",
               "x": 0, "z": 0, "rot": 0, "w": 1, "d": 1}],  // <= 512, w and d only for bed
    "trees": {"mode": "edge|scatter", "band": 3.2, "seed": 23},
    "fence": true
  },
  "pavilions": [
    {"id": "p1", "type": "lobby|cabin|research|workspace|treehouse|cafe|play",
     "x": 0, "z": 0, "rot": 0,
     "account": "<account id>",         // cabin only, required there
     "label": "WORK",                   // optional, <= 40 chars, cleaned
     "cols": 2, "rows": 1}              // desks, cabin/research/workspace only, 1..8 each
  ],                                    // <= 64; exactly one lobby; at most one treehouse
  "overflow": [{"x": 0, "z": 0, "rot": 0, "near": "workspace|cafe|play|lobby"}],  // <= 64 standing spots
  "eye": {"from": [x,z], "look": [x,z]} // eye level camera, optional
}
```

pavilion footprints (w x d before rotation) come from one table,
internal/office/layout/data/footprints.json (see shared files), used by the go
overlap check and the page builders so they cannot drift. the treehouse holds
the review and meeting rooms on one raised deck with stairs.

rules go enforces, refusing instead of repairing:
- unknown keys rejected, strings cleaned, no urls, file cap 512 KB
- pavilions inside the base outline (all four corners), pavilion footprints do
  not overlap each other, the river or a pond
- ids [a-z0-9-]{1,32} unique, exactly one lobby, cabins name an account
- a template layout is never stored: only its name is (active.json), the layout
  is generated on each read

routing on v2 (page, pure functions in web/js/routing): cabin -> that account's
main sessions; research -> Explore; treehouse review side -> reviewers,
treehouse meeting side -> Plan; workspace -> general-purpose and unknown types;
cafe and play -> idle breaks; lobby -> arrivals; overflow spots when a room is
full, then the lobby.

## generators (go, internal/office/layout/garden)

`Generate(template string, in Input) (Layout, error)`
- Input: accounts (ordered id and label, 0..12), per account cabin desks
  (at least 2), workspace desks (at least 8), research desks (at least 4).
- desks round up to whole rows; rows max 4 per pavilion, extra desks add a
  second workspace pavilion.
- rows: cabins in a row along the back, wraps to a second row after 6.
- village: pavilions on an ellipse ring facing a green with the pond and cafe;
  the ring radius grows with the pavilion count; second ring after 10.
- river: cabins along the west bank, river and island lengthen with accounts.
- street: one winding street from the gate to a fountain plaza, houses on both
  sides; the street lengthens with accounts; a side street after 8.
- every output passes Validate; the generator tests run 0..12 accounts and
  desk needs up to 40 for each template and require a valid layout.
- `Tracker` holds the needs with the 30 s grow and 10 min shrink holds; the
  server feeds it from each snapshot and serves the generated layout with a
  `rev` header so the page rebuilds only when it changes.

## shared files (identical bytes in every stream that needs them)

internal/office/layout/types_v2.go, owned by stream 1, copied verbatim by
stream 2:

```go
package layout

// Site is the ground of a v2 layout: the island, water, paths and planting.
type Site struct {
	Base    Poly     `json:"base"`
	Land    []Poly   `json:"land"`
	River   *River   `json:"river"`
	Ponds   []Pond   `json:"ponds"`
	Paths   []Path   `json:"paths"`
	Bridges []Bridge `json:"bridges"`
	Props   []Prop   `json:"props"`
	Trees   Trees    `json:"trees"`
	Fence   bool     `json:"fence"`
}

// Pt is an [x, z] point in world units.
type Pt [2]float64

// Poly is a closed outline.
type Poly struct {
	Poly []Pt `json:"poly"`
}

// River is a stream along a centre line.
type River struct {
	Pts []Pt    `json:"pts"`
	W   float64 `json:"w"`
}

// Pond is an oval pond.
type Pond struct {
	X  float64 `json:"x"`
	Z  float64 `json:"z"`
	RX float64 `json:"rx"`
	RZ float64 `json:"rz"`
}

// Path is a walkway along a polyline.
type Path struct {
	Pts    []Pt    `json:"pts"`
	W      float64 `json:"w"`
	Smooth bool    `json:"smooth"`
}

// Bridge crosses water from a to b.
type Bridge struct {
	A Pt      `json:"a"`
	B Pt      `json:"b"`
	W float64 `json:"w"`
}

// Prop is a small site object.
type Prop struct {
	T   string  `json:"t"`
	X   float64 `json:"x"`
	Z   float64 `json:"z"`
	Rot float64 `json:"rot"`
	W   float64 `json:"w,omitempty"`
	D   float64 `json:"d,omitempty"`
}

// Trees says how the page plants trees.
type Trees struct {
	Mode string  `json:"mode"`
	Band float64 `json:"band"`
	Seed int     `json:"seed"`
}

// Pavilion is one building of a v2 layout.
type Pavilion struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Rot     float64 `json:"rot"`
	Account string  `json:"account,omitempty"`
	Label   string  `json:"label,omitempty"`
	Cols    int     `json:"cols,omitempty"`
	Rows    int     `json:"rows,omitempty"`
}

// Spot is a standing place for overflow agents.
type Spot struct {
	X    float64 `json:"x"`
	Z    float64 `json:"z"`
	Rot  float64 `json:"rot"`
	Near string  `json:"near"`
}

// Eye is the eye level camera.
type Eye struct {
	From Pt `json:"from"`
	Look Pt `json:"look"`
}
```

Layout gains these fields (stream 1 edits types.go; v1 fields stay, omitempty
where a v2 file does not use them, and the decoder picks the rules by "v"):
`Template string json:"template,omitempty"`, `Site *Site json:"site,omitempty"`,
`Pavilions []Pavilion json:"pavilions,omitempty"`, `Overflow []Spot
json:"overflow,omitempty"`, `Eye *Eye json:"eye,omitempty"`. Version becomes 2
and V1 = 1 is added. stream 2 builds against these names and adds them to its
own copy of types.go the same way; stream 1's types.go wins at integration.

internal/office/layout/data/footprints.json, owned by stream 1, read by the
page (stream 3 keeps a copy of the same numbers as its embedded fallback):

```json
{"lobby": {"w": 3.8, "d": 4.0}, "cafe": {"w": 5.6, "d": 4.0},
 "play": {"w": 5.0, "d": 4.0}, "treehouse": {"w": 9.6, "d": 4.2},
 "cabin": {"col": 2.1, "row": 2.8, "pad": 0.0},
 "research": {"col": 2.1, "row": 3.5, "pad": 0.2},
 "workspace": {"col": 2.1, "row": 3.5, "pad": 0.2}}
```
desk pavilions are col*cols + pad wide and row*rows + pad deep.

## work split (one pr, four parallel streams, then integration)

1. go format: v2 types, decode (v1 or v2 by "v"), validate, footprints.json,
   store and routes (template query, --template flag, active template names).
2. go generators and tracker: internal/office/layout/garden.
3. web garden renderer: port the viewer modules into web/js/garden/, apply.js
   dispatches v1 to the hall and v2 to the garden, cameras and presets.
4. web routing and ui: seats from v2 pavilions, overflow spots, template
   switcher, camera bar, url hash, node tests.

## open

- real-gpu frame rate with several hundred trees and a 4096 shadow map is not
  measured yet; the renderer must cap tree count and shadow size.
