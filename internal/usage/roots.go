package usage

import "path/filepath"

// CollectRoots de-duplicates a list of candidate transcript root directories by
// real path, so a root that is a symlink to another one in the list is never
// scanned twice. clalt lists only the projects folder of the shared claude home,
// which the projects folder of every account links to, but the rule is the same
// for any list. A real (non-symlinked) dir is kept as a distinct root. A missing
// dir is kept too (ScanProjects already tolerates that, yielding no records for
// it) rather than silently dropped, since dropping it here would be
// indistinguishable from a real scan error.
func CollectRoots(dirs []string) []string {
	seen := make(map[string]bool, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		key := dir
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			key = real
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, dir)
	}
	return out
}

// ScanRoots scans every root (see CollectRoots) and returns the combined,
// deduped record set. Dedup (see Dedup) applies across all roots together, so
// a resumed session visible under more than one root still counts once.
func ScanRoots(roots []string) ([]Record, error) {
	var all []Record
	for _, root := range CollectRoots(roots) {
		recs, err := ScanProjects(root)
		if err != nil {
			return nil, err
		}
		all = append(all, recs...)
	}
	return Dedup(all), nil
}
