package conversation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testID = "11111111-2222-3333-4444-555555555555"

// write makes a transcript of the given lines in a project folder under a temp
// projects root and returns the root.
func write(t *testing.T, project, id string, lines ...string) (root, path string) {
	t.Helper()
	root = t.TempDir()
	return root, writeIn(t, root, project, id, lines...)
}

func writeIn(t *testing.T, root, project, id string, lines ...string) string {
	t.Helper()
	dir := filepath.Join(root, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func user(text string) string {
	return `{"type":"user","timestamp":"2026-10-20T10:00:00Z","cwd":"/w/app","message":{"role":"user","content":"` + text + `"}}`
}

func describeOne(t *testing.T, project string, lines ...string) Conversation {
	t.Helper()
	_, path := write(t, project, testID, lines...)
	c, ok, err := describe(path, project)
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	return c
}

func TestTitleOrderIsNameThenGeneratedThenSummaryThenFirstPrompt(t *testing.T) {
	prompt := user("fix the login bug")
	ai := `{"type":"ai-title","aiTitle":"Login fix","sessionId":"x"}`
	summary := `{"type":"summary","summary":"Old summary","leafUuid":"u"}`
	name := `{"type":"custom-title","customTitle":"my name","sessionId":"x"}`
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"first prompt only", []string{prompt}, "fix the login bug"},
		{"summary beats prompt", []string{prompt, summary}, "Old summary"},
		{"generated beats summary", []string{prompt, summary, ai}, "Login fix"},
		{"name beats generated", []string{prompt, ai, name}, "my name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describeOne(t, "-w-app", c.lines...).Title; got != c.want {
				t.Errorf("title = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTheLastTitleRecordOfAKindWins(t *testing.T) {
	c := describeOne(t, "-w-app", user("p"),
		`{"type":"ai-title","aiTitle":"first"}`, `{"type":"ai-title","aiTitle":"second"}`)

	if c.Title != "second" {
		t.Errorf("title = %q", c.Title)
	}
}

func TestFirstPromptSkipsWhatIsNotAPrompt(t *testing.T) {
	c := describeOne(t, "-w-app",
		`{"type":"user","isMeta":true,"message":{"content":"meta caveat"}}`,
		`{"type":"user","isSidechain":true,"message":{"content":"sidechain"}}`,
		`{"type":"user","isCompactSummary":true,"message":{"content":"compact summary"}}`,
		`{"type":"user","origin":{"kind":"task-notification"},"message":{"content":"injected"}}`,
		`{"type":"user","message":{"content":"<command-name>/clear</command-name>"}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"result"}]}}`,
		`{"type":"user","message":{"content":"   "}}`,
		`{"type":"user","origin":{"kind":"human"},"message":{"content":[{"type":"image"},{"type":"text","text":"the real  \n prompt"}]}}`,
		user("a later prompt"))

	if c.Title != "the real prompt" {
		t.Errorf("title = %q", c.Title)
	}
}

func TestALongFirstPromptIsCutAtAWordWithDots(t *testing.T) {
	c := describeOne(t, "-w-app", user(strings.Repeat("word ", 60)))

	if !strings.HasSuffix(c.Title, "...") || len([]rune(c.Title)) > 103 || strings.Contains(c.Title, "  ") {
		t.Errorf("title = %q (%d runes)", c.Title, len([]rune(c.Title)))
	}
	if strings.HasSuffix(strings.TrimSuffix(c.Title, "..."), " ") {
		t.Errorf("title %q ends in a space before the dots", c.Title)
	}
}

func TestACutNeverSplitsACharacter(t *testing.T) {
	c := describeOne(t, "-w-app", user(strings.Repeat("\u00e9", 300)))

	if !strings.HasSuffix(c.Title, "...") || strings.ToValidUTF8(c.Title, "?") != c.Title {
		t.Errorf("title = %q", c.Title)
	}
}

func TestCwdIsTheOneThatEncodesToTheFolderName(t *testing.T) {
	cwdLine := func(cwd string) string {
		return `{"type":"user","cwd":"` + cwd + `","message":{"content":"x"}}`
	}
	c := describeOne(t, "-Users-me-app", cwdLine("/Users/me"), cwdLine("/Users/me/app"), cwdLine("/Users/me/other"))

	if c.Cwd != "/Users/me/app" {
		t.Errorf("cwd = %q, want the one that encodes to the folder", c.Cwd)
	}
}

func TestCwdFallsBackToTheFirstOne(t *testing.T) {
	cwdLine := func(cwd string) string {
		return `{"type":"user","cwd":"` + cwd + `","message":{"content":"x"}}`
	}
	c := describeOne(t, "-nothing-matches", cwdLine("/first"), cwdLine("/second"))

	if c.Cwd != "/first" {
		t.Errorf("cwd = %q", c.Cwd)
	}
}

func TestModelIsTheLastRealOne(t *testing.T) {
	asst := func(model string) string {
		return `{"type":"assistant","timestamp":"2026-10-20T10:01:00Z","message":{"model":"` + model + `","content":[]}}`
	}
	c := describeOne(t, "-w-app", user("p"), asst("claude-sonnet-5"), asst("claude-opus-5-5"), asst("<synthetic>"))

	if c.Model != "claude-opus-5-5" {
		t.Errorf("model = %q", c.Model)
	}
}

func TestLastActiveIsTheNewestTimestamp(t *testing.T) {
	c := describeOne(t, "-w-app",
		`{"type":"user","timestamp":"2026-10-20T10:00:00Z","message":{"content":"p"}}`,
		`{"type":"assistant","timestamp":"2026-10-21T08:30:00.250Z","message":{"model":"m"}}`,
		`{"type":"last-prompt","lastPrompt":"p"}`)

	want := time.Date(2026, 10, 21, 8, 30, 0, 250_000_000, time.UTC)
	if !c.LastActive.Equal(want) {
		t.Errorf("last active = %v, want %v", c.LastActive, want)
	}
}

func TestLastActiveFallsBackToTheFileTime(t *testing.T) {
	_, path := write(t, "-w-app", testID, `{"type":"user","message":{"content":"p"}}`)
	when := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}

	c, _, _ := describe(path, "-w-app")

	if !c.LastActive.Equal(when) {
		t.Errorf("last active = %v, want the mtime %v", c.LastActive, when)
	}
}

func TestSizeAndEntrypointAndIdentity(t *testing.T) {
	_, path := write(t, "-w-app", testID, `{"type":"user","entrypoint":"sdk-cli","message":{"content":"p"}}`)

	c, _, _ := describe(path, "-w-app")

	fi, _ := os.Stat(path)
	if c.ID != testID || c.Project != "-w-app" || c.Path != path || c.Size != fi.Size() || c.Entrypoint != "sdk-cli" {
		t.Errorf("conversation = %+v", c)
	}
}

func TestMalformedAndOversizedLinesAreSkipped(t *testing.T) {
	huge := `{"type":"user","message":{"content":"` + strings.Repeat("x", maxLine+10) + `"}}`
	c := describeOne(t, "-w-app", `not json`, `{"type":`, huge, user("survivor"),
		`{"type":"ai-title","aiTitle":"after the huge line"}`)

	if c.Title != "after the huge line" {
		t.Errorf("title = %q, the records after a bad line were lost", c.Title)
	}
}

func TestAFileWithoutMessagesIsNotAConversation(t *testing.T) {
	_, path := write(t, "-w-app", testID, `{"type":"file-history-snapshot"}`, `{"type":"last-prompt","lastPrompt":"x"}`)

	_, ok, err := describe(path, "-w-app")

	if ok || err != nil {
		t.Errorf("ok=%v err=%v", ok, err)
	}
}

func TestATypeMismatchInOneFieldKeepsTheRest(t *testing.T) {
	c := describeOne(t, "-w-app", `{"type":"user","isMeta":"yes","cwd":"/x","message":{"content":"still read"}}`)

	if c.Cwd != "/x" {
		t.Errorf("cwd = %q", c.Cwd)
	}
}
