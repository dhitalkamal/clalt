package demo

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the demo is shown to other people, so neither it nor its tests, goldens and
// plans may carry names from anyone's real work. the words are built from
// pieces so this file does not match itself.
func TestNoWorkIdentifiersAnywhereInTheDashboardOrItsPlans(t *testing.T) {
	forbidden := []string{"recruit" + "able", "rhr" + "-", "rhr" + "_"}
	roots := []string{"..", "../../cli", "../../../docs/plans"}
	scanned := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(path) {
			case ".go", ".txt", ".md":
			default:
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for i, line := range strings.Split(strings.ToLower(string(raw)), "\n") {
				for _, word := range forbidden {
					if strings.Contains(line, word) {
						t.Errorf("%s:%d has %q", path, i+1, word)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 50 {
		t.Errorf("the walk should reach the sources, goldens and plans, it saw %d files", scanned)
	}
}
