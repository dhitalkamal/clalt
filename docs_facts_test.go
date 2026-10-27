package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/dhitalkamal/clalt/internal/claudecli"
)

// goVersionOfModule is the version the go line of go.mod asks for.
func goVersionOfModule(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if v, ok := strings.CutPrefix(line, "go "); ok {
			return strings.TrimSpace(v)
		}
	}
	t.Fatal("go.mod has no go line")
	return ""
}

// the readme tells people which Go to build with, and it has to be the one that
// go.mod asks for. an older Go starts, finds the line, and tries to fetch the
// right one.
func TestTheReadmeNamesTheGoVersionThatGoModAsksFor(t *testing.T) {
	want := "Go " + goVersionOfModule(t)
	if readme := strings.Join(lines(t, "README.md"), "\n"); !strings.Contains(readme, want) {
		t.Errorf("README.md does not say it needs %s", want)
	}
}

var envName = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)

// codeBlocks are the fenced blocks of a markdown file, each as the list of the
// variable names in it.
func codeBlocks(t *testing.T, path string) [][]string {
	t.Helper()
	var blocks [][]string
	var current []string
	in := false
	for _, line := range lines(t, path) {
		switch {
		case strings.HasPrefix(line, "```") && !in:
			in, current = true, nil
		case strings.HasPrefix(line, "```") && in:
			in = false
			blocks = append(blocks, current)
		case in:
			current = append(current, envName.FindAllString(line, -1)...)
		}
	}
	return blocks
}

// auth.md says that twelve of the removed variables outrank a subscription
// login in claude's precedence and that five more are removed for other
// reasons. every name in it has to be one that the code really removes.
func TestAuthDocListsTwelveVariablesThatOutrankTheLoginAndFiveMore(t *testing.T) {
	blocks := codeBlocks(t, "docs/auth.md")
	if len(blocks) != 2 || len(blocks[0]) != 12 || len(blocks[1]) != 5 {
		t.Fatalf("want two blocks of 12 and 5 variables, got %v", blocks)
	}
	for _, block := range blocks {
		for _, name := range block {
			if !claudecli.IsAuthEnvVar(name) {
				t.Errorf("docs/auth.md lists %s, which clalt does not remove", name)
			}
		}
	}
}
