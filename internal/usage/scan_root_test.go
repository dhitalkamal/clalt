package usage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const scanLine = `{"type":"assistant","timestamp":"2026-08-24T10:00:00.000Z","sessionId":"s1","requestId":"req_1","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":10,"output_tokens":20}}}` + "\n"

// projectsWithOneTranscript makes a projects folder with one reply in it.
func projectsWithOneTranscript(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "projects")
	if err := os.MkdirAll(filepath.Join(root, "proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proj", "s.jsonl"), []byte(scanLine), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// the projects folder of an account is a link to the shared one, and a walk does
// not descend into a link at its root: it has to be followed first, or every
// account folder would read as having no usage at all.
func TestScanProjectsFollowsALinkAtTheRoot(t *testing.T) {
	root := projectsWithOneTranscript(t)
	link := filepath.Join(t.TempDir(), "projects")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	recs, err := ScanProjects(link)

	if err != nil || len(recs) != 1 {
		t.Errorf("ScanProjects(link) = %d records, %v; want the one reply", len(recs), err)
	}
}

func TestScanProjectsFollowsALinkOfLinks(t *testing.T) {
	root := projectsWithOneTranscript(t)
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first"), filepath.Join(dir, "second")
	if err := os.Symlink(root, first); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(first, second); err != nil {
		t.Fatal(err)
	}

	recs, err := ScanProjects(second)

	if err != nil || len(recs) != 1 {
		t.Errorf("ScanProjects = %d records, %v", len(recs), err)
	}
}

// a machine that has never saved a transcript has no projects folder, which is
// no usage and not a problem
func TestScanProjectsOfAFolderThatIsNotThereIsNoUsage(t *testing.T) {
	recs, err := ScanProjects(filepath.Join(t.TempDir(), "never-made"))

	if err != nil || len(recs) != 0 {
		t.Errorf("ScanProjects = %d records, %v; want none and no error", len(recs), err)
	}
}

// a link to a place that is not there is a volume that is not mounted. that is
// not "no usage", it is usage that cannot be read, and the reader has to say so.
func TestScanProjectsSaysWhenALinkLeadsNowhere(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "projects")
	if err := os.Symlink(filepath.Join(dir, "volume-not-mounted", "projects"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	recs, err := ScanProjects(link)

	if err == nil || len(recs) != 0 || !strings.Contains(err.Error(), link) {
		t.Errorf("ScanProjects = %d records, %v; want an error that names the link", len(recs), err)
	}
}

// the walk skips what it cannot read below the root, but the root itself is the
// whole read: when that cannot be read the numbers are not zero, they are
// missing
func TestScanProjectsReturnsTheErrorOfTheRootItself(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("no permission bits to take away")
	}
	root := projectsWithOneTranscript(t)
	if err := os.Chmod(root, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	recs, err := ScanProjects(root)

	if err == nil || len(recs) != 0 {
		t.Errorf("ScanProjects = %d records, %v; want an error", len(recs), err)
	}
}

func TestScanProjectsStillSkipsAFolderBelowTheRootThatCannotBeRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("no permission bits to take away")
	}
	root := projectsWithOneTranscript(t)
	locked := filepath.Join(root, "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "x.jsonl"), []byte(scanLine), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	recs, err := ScanProjects(root)

	if err != nil || len(recs) != 1 {
		t.Errorf("ScanProjects = %d records, %v; want the readable one and no error", len(recs), err)
	}
}

func TestScanRootsReturnsTheErrorOfARoot(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "projects")
	if err := os.Symlink(filepath.Join(dir, "gone"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := ScanRoots([]string{projectsWithOneTranscript(t), link}); err == nil {
		t.Error("ScanRoots hid a root that could not be read")
	}
}
