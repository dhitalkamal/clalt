package office

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func agentFile(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\nBODY-SECRET do not read\n"
}

func names(defs []AgentDef) string {
	var out []string
	for _, d := range defs {
		out = append(out, d.Name+"="+d.Desc)
	}
	return strings.Join(out, "|")
}

func TestReadAgentDirKeepsOnlyNameAndDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", agentFile("alpha", "does alpha things"))
	writeFile(t, dir, "q.md", "---\nname: \"quoted\"\ndescription: 'single quoted'\n---\n")
	writeFile(t, dir, "block.md", "---\nname: block\ndescription: >\n  first line\n  second line\nmodel: opus\n---\n")
	writeFile(t, dir, "esc.md", "---\nname: esc\ndescription: \"one \\\"two\\\" \\n\\nExamples: x\"\n---\n")
	writeFile(t, dir, "nodesc.md", "---\nname: nodesc\n---\n")
	writeFile(t, dir, "noname.md", "---\ndescription: lost\n---\n")
	writeFile(t, dir, "nofront.md", "name: plain\n")
	writeFile(t, dir, "open.md", "---\nname: open\ndescription: never closed\n")
	writeFile(t, dir, "crlf.md", "---\r\nname: crlf\r\ndescription: windows\r\n---\r\n")
	writeFile(t, dir, "bak.md.pre-123", agentFile("backup", "old"))
	writeFile(t, dir, "notes.txt", agentFile("text", "not md"))
	writeFile(t, dir, "UP.MD", agentFile("upper", "wrong case"))
	if err := os.Mkdir(filepath.Join(dir, "sub.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := names(readAgentDir(dir, "user"))
	want := "alpha=does alpha things|block=first line second line|crlf=windows|esc=one \"two\" Examples: x|nodesc=|quoted=single quoted"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	for _, d := range readAgentDir(dir, "user") {
		if d.Source != "user" || strings.Contains(d.Name+d.Desc, "BODY-SECRET") {
			t.Errorf("bad def %+v", d)
		}
	}
}

func TestReadAgentDirCaps(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 70; i++ {
		writeFile(t, dir, fmt.Sprintf("a%02d.md", i), agentFile(fmt.Sprintf("agent%02d", i), "x"))
	}
	if n := len(readAgentDir(dir, "user")); n != 64 {
		t.Errorf("%d files read, want 64", n)
	}
	big := t.TempDir()
	writeFile(t, big, "big.md", agentFile("big", "x")+strings.Repeat("a", 64*1024))
	writeFile(t, big, "ok.md", agentFile("ok", "x"))
	if got := names(readAgentDir(big, "user")); got != "ok=x" {
		t.Errorf("oversize file not skipped: %q", got)
	}
}

func TestReadAgentDirSymlinks(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, outside, "secret.md", agentFile("outside", "must not be read"))
	dir := t.TempDir()
	writeFile(t, dir, "real.md", agentFile("real", "inside"))
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(dir, "out.md")); err != nil {
		t.Skip("no symlinks:", err)
	}
	if err := os.Symlink(filepath.Join(dir, "real.md"), filepath.Join(dir, "in.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing.md"), filepath.Join(dir, "dangling.md")); err != nil {
		t.Fatal(err)
	}
	if got := names(readAgentDir(dir, "project")); got != "real=inside|real=inside" {
		t.Errorf("got %q, want the real file and its inside link only", got)
	}
	// a symlinked folder is the user's own choice and is followed
	link := filepath.Join(t.TempDir(), "agents")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if got := names(readAgentDir(link, "user")); got != "real=inside|real=inside" {
		t.Errorf("symlinked folder: %q", got)
	}
}

func TestReadAgentDirMissingIsEmpty(t *testing.T) {
	if got := readAgentDir(filepath.Join(t.TempDir(), "none"), "user"); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestTeamFilesReadsUserAndProjectFoldersAndCachesFor30s(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	userDir := filepath.Join(home, "agents")
	writeFile(t, userDir, "u.md", agentFile("uagent", "from user"))
	writeFile(t, filepath.Join(proj, ".claude", "agents"), "p.md", agentFile("pagent", "from project"))
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	tf := NewTeamFiles(userDir, func() time.Time { return now })

	cwds := []string{proj, proj, "relative/dir", "", filepath.Join(t.TempDir(), "gone")}
	var got []string
	for _, d := range tf.Agents(cwds) {
		got = append(got, d.Source+":"+d.Name)
	}
	if strings.Join(got, ",") != "user:uagent,project:pagent" {
		t.Fatalf("got %v", got)
	}

	writeFile(t, userDir, "u2.md", agentFile("second", "new"))
	now = now.Add(29 * time.Second)
	if n := len(tf.Agents(cwds)); n != 2 {
		t.Errorf("re-read after 29s: %d defs", n)
	}
	now = now.Add(2 * time.Second)
	if n := len(tf.Agents(cwds)); n != 3 {
		t.Errorf("no re-read after 31s: %d defs", n)
	}
}

func TestTeamFilesWithoutAUserDirOrWithTheUserDirAsProject(t *testing.T) {
	home := t.TempDir()
	userDir := filepath.Join(home, ".claude", "agents")
	writeFile(t, userDir, "u.md", agentFile("uagent", "x"))
	tf := NewTeamFiles(userDir, nil)
	// a session started in the home folder must not list the user agents twice
	if got := tf.Agents([]string{home}); len(got) != 1 {
		t.Errorf("home as project: %v", got)
	}
	if got := NewTeamFiles("", nil).Agents([]string{home}); len(got) != 1 || got[0].Source != "project" {
		t.Errorf("no user dir: %v", got)
	}
}
