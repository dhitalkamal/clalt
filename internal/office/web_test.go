package office

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

func embedded(t *testing.T) []string {
	t.Helper()
	var files []string
	err := fs.WalkDir(webFS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func read(t *testing.T, p string) string {
	t.Helper()
	raw, err := fs.ReadFile(webFS(), p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestEmbedHoldsThePageTheLicensesAndNoDemoLeftovers(t *testing.T) {
	have := map[string]bool{}
	for _, f := range embedded(t) {
		have[f] = true
		for _, stray := range []string{"fake", "shots", "selfcheck", "frames", "serve.sh", "README", ".py", ".bak", ".map", ".DS_Store", "node_modules"} {
			if strings.Contains(f, stray) {
				t.Errorf("stray demo file embedded: %s", f)
			}
		}
	}
	for _, f := range []string{"index.html", "app.css", "CREDITS.txt", "vendor/three/LICENSE", "vendor/three/three.module.min.js",
		"vendor/three/examples/jsm/loaders/GLTFLoader.js", "assets/characters/Textures/colormap.png", "js/main.js", "js/sync.js", "js/poll.js"} {
		if !have[f] {
			t.Errorf("%s is not embedded", f)
		}
	}
}

func TestEveryEmbeddedGLBIsLoadedByThePage(t *testing.T) {
	assets := read(t, "js/assets.js")
	for _, f := range embedded(t) {
		if !strings.HasSuffix(f, ".glb") {
			continue
		}
		name := strings.TrimSuffix(path.Base(f), ".glb")
		name = strings.TrimPrefix(name, "character-")
		if !strings.Contains(assets, "'"+name+"'") {
			t.Errorf("%s is embedded but js/assets.js never names it", f)
		}
	}
}

var importRe = regexp.MustCompile(`from '\./([a-zA-Z]+)\.js'`)

func TestEveryJSModuleIsImportedByAnotherAndStaysUnder300Lines(t *testing.T) {
	imported := map[string]bool{"main": true}
	var mods []string
	for _, f := range embedded(t) {
		if !strings.HasPrefix(f, "js/") {
			continue
		}
		src := read(t, f)
		mods = append(mods, f)
		if n := strings.Count(src, "\n"); n >= 300 {
			t.Errorf("%s has %d lines, the limit is under 300", f, n)
		}
		for _, m := range importRe.FindAllStringSubmatch(src, -1) {
			imported[m[1]] = true
		}
	}
	for _, f := range mods {
		if !imported[strings.TrimSuffix(path.Base(f), ".js")] {
			t.Errorf("%s is never imported", f)
		}
	}
}

func TestOurJSHTMLAndCSSAreAscii(t *testing.T) {
	for _, f := range embedded(t) {
		if !(strings.HasPrefix(f, "js/") || f == "index.html" || f == "app.css") {
			continue
		}
		for i, line := range strings.Split(read(t, f), "\n") {
			for _, r := range line {
				if r > 127 {
					t.Errorf("%s:%d has a non-ascii character %U", f, i+1, r)
					break
				}
			}
		}
	}
}

func TestPageHasNoDemoControlsOrTestHooks(t *testing.T) {
	page := read(t, "index.html")
	for _, id := range []string{"b-spawn", "b-finish", "b-add", "b-remove", "b-cycle", "b-reset", "empty-add"} {
		if strings.Contains(page, id) {
			t.Errorf("index.html still has the demo control %s", id)
		}
	}
	for _, id := range []string{"b-top", "b-front", "b-home"} {
		if !strings.Contains(page, id) {
			t.Errorf("index.html lost the camera button %s", id)
		}
	}
	for _, f := range embedded(t) {
		if !strings.HasPrefix(f, "js/") {
			continue
		}
		src := read(t, f)
		for _, hook := range []string{"window.__", "__selfCheck", "?frames", "params.has('frames')"} {
			if strings.Contains(src, hook) {
				t.Errorf("%s keeps the test hook %q", f, hook)
			}
		}
	}
}

func TestPageHasNoInlineScriptOrStyleExceptTheImportMap(t *testing.T) {
	page := read(t, "index.html")
	if strings.Contains(page, "<style") || strings.Contains(page, " style=") {
		t.Error("inline style is blocked by the CSP, use app.css")
	}
	for _, m := range regexp.MustCompile(`(?s)<script([^>]*)>`).FindAllStringSubmatch(page, -1) {
		if !strings.Contains(m[1], "src=") && !strings.Contains(m[1], "importmap") {
			t.Errorf("inline script: %s", m[0])
		}
	}
}
