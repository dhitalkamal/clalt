// Package layout is the floor plan of the office: the json format the page
// draws and edits, its validation, and the store of layouts on disk. a layout
// file is untrusted input, so everything here refuses instead of repairing.
package layout

// Version is the only layout format there is.
const Version = 1

// limits of the format, in cells unless said otherwise.
const (
	MaxBytes     = 512 * 1024
	MinSide      = 8
	MaxSide      = 120
	MaxRooms     = 64
	MaxFurniture = 1500
	MaxTitle     = 60
	MaxLabel     = 40
	// GridSize is the world units per cell, fixed in v1.
	GridSize = 0.5
)

// Layout is one office floor plan.
type Layout struct {
	V          int     `json:"v"`
	Name       string  `json:"name"`
	Title      string  `json:"title"`
	Grid       float64 `json:"grid"`
	Size       Size    `json:"size"`
	Walls      Walls   `json:"walls"`
	FloorColor string  `json:"floorColor"`
	Door       Door    `json:"door"`
	Deck       *Rect   `json:"deck"`
	Stairs     *Stairs `json:"stairs"`
	Rooms      []Room  `json:"rooms"`
	Furniture  []Furn  `json:"furniture"`
}

// Size is the ground floor in cells.
type Size struct {
	W int `json:"w"`
	D int `json:"d"`
}

// Walls holds the wall look.
type Walls struct {
	Color string `json:"color"`
}

// Door is the entrance: a side and a cell offset along it.
type Door struct {
	Side string `json:"side"`
	At   int    `json:"at"`
}

// Rect is an area in cells, x and z are its minimum corner.
type Rect struct {
	X int `json:"x"`
	Z int `json:"z"`
	W int `json:"w"`
	D int `json:"d"`
}

// Stairs is the stair cell on the ground floor and the way it climbs.
type Stairs struct {
	X   int    `json:"x"`
	Z   int    `json:"z"`
	Dir string `json:"dir"`
}

// Room is a typed area of one level.
type Room struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Level   int    `json:"level"`
	X       int    `json:"x"`
	Z       int    `json:"z"`
	W       int    `json:"w"`
	D       int    `json:"d"`
	Label   string `json:"label"`
	Floor   string `json:"floor,omitempty"`
	Account string `json:"account,omitempty"`
}

// Furn is one placed catalog item. x and z are the minimum corner of its
// footprint after rotation.
type Furn struct {
	ID    string `json:"id"`
	Item  string `json:"item"`
	Level int    `json:"level"`
	X     int    `json:"x"`
	Z     int    `json:"z"`
	Rot   int    `json:"rot"`
	Room  string `json:"room,omitempty"`
}

// roomTypes are the types a room can have.
var roomTypes = map[string]bool{
	"lobby": true, "cabins": true, "research": true, "workspace": true,
	"review": true, "meeting": true, "cafe": true, "play": true,
}
