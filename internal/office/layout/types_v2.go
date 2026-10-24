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
