package office

import (
	"embed"
	"io/fs"
)

//go:embed web
var webRoot embed.FS

// webFS is the embedded page, rooted at the web folder. it is all the server
// ever serves besides the snapshot.
func webFS() fs.FS {
	sub, err := fs.Sub(webRoot, "web")
	if err != nil {
		panic("office: embedded web folder missing: " + err.Error())
	}
	return sub
}
