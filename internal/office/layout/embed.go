package layout

import "embed"

// embeddedFS holds data/catalog.json and data/templates/*.json, which the page
// builder writes. the folder always has a README so the package builds while
// the other files are missing.
//
//go:embed data
var embeddedFS embed.FS
