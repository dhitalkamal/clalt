package layout

// Template is one entry of the template list the page and the cli offer.
type Template struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// HallTemplate is the old v1 hall, served from the embedded default layout.
const HallTemplate = "hall"

// DefaultActive is the template shown when active.json names nothing.
const DefaultActive = HallTemplate

// Templates are the names that answer a read without a stored file: four
// generated gardens and the hall. a user layout cannot take these names.
var Templates = []Template{
	{"rows", "Rows"}, {"village", "Village"}, {"river", "River"}, {"street", "Street"}, {HallTemplate, "Hall"},
}

// IsTemplate says whether id is one of Templates.
func IsTemplate(id string) bool {
	for _, t := range Templates {
		if t.ID == id {
			return true
		}
	}
	return false
}
