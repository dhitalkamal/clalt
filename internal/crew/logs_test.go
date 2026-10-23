package crew

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

func writeLog(t *testing.T, body string) string {
	t.Helper()
	isolate(t)
	p, _ := PathsFor("work")
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Log, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p.Log
}

func TestTailGivesTheLastLinesInOrder(t *testing.T) {
	path := writeLog(t, "one\ntwo\nthree\nfour\n")
	got, err := Tail(path, 2)
	if err != nil || !slices.Equal(got, []string{"three", "four"}) {
		t.Errorf("Tail = %v, %v", got, err)
	}
}

func TestTailOfAShortFileGivesAllOfIt(t *testing.T) {
	path := writeLog(t, "one\ntwo")
	got, err := Tail(path, 10)
	if err != nil || !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("Tail = %v, %v", got, err)
	}
}

func TestTailOfAnEmptyFileIsNothing(t *testing.T) {
	path := writeLog(t, "")
	if got, err := Tail(path, 5); err != nil || len(got) != 0 {
		t.Errorf("Tail = %v, %v", got, err)
	}
}

func TestTailOfAHugeFileReadsOnlyTheEndAndKeepsLinesWhole(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200000; i++ {
		fmt.Fprintf(&b, "line-%d some text to make it longer\n", i)
	}
	path := writeLog(t, b.String())
	got, err := Tail(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"line-199997 some text to make it longer", "line-199998 some text to make it longer", "line-199999 some text to make it longer",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Tail = %v", got)
	}
	// asking for more than the window holds still gives whole lines only
	many, err := Tail(path, 1000000)
	if err != nil || len(many) == 0 || len(many) >= 200000 {
		t.Fatalf("got %d lines, %v", len(many), err)
	}
	for _, line := range many {
		if !strings.HasPrefix(line, "line-") || !strings.HasSuffix(line, "longer") {
			t.Fatalf("a line was cut: %q", line)
		}
	}
}

func TestTailHidesTokenValuesAndNothingElse(t *testing.T) {
	path := writeLog(t, strings.Join([]string{
		"GET /api/ws?token=abc123def&x=1 200",
		"open http://localhost:5480/?TOKEN=Zz9-_.~ now",
		"Authorization: Bearer eyJhbGciOi.payload.sig",
		"access_token=sekret and session_key=keep-this",
		"nothing to hide here",
	}, "\n"))
	got, err := Tail(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /api/ws?token=<hidden>&x=1 200",
		"open http://localhost:5480/?TOKEN=<hidden> now",
		"Authorization: Bearer <hidden>",
		"access_token=<hidden> and session_key=keep-this",
		"nothing to hide here",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Tail =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTailOfALogThatIsNotThereSaysSo(t *testing.T) {
	isolate(t)
	p, _ := PathsFor("work")
	if _, err := Tail(p.Log, 5); !errors.Is(err, ErrNoLog) {
		t.Errorf("err = %v, want ErrNoLog", err)
	}
}

func TestLogsOfAnAccountReadItsOwnLog(t *testing.T) {
	writeLog(t, "a\nb\n")
	m := NewManager(Options{})
	if got, err := m.Logs("work", 1); err != nil || !slices.Equal(got, []string{"b"}) {
		t.Errorf("Logs = %v, %v", got, err)
	}
	if _, err := m.Logs("nobody", 1); !errors.Is(err, ErrNoLog) {
		t.Errorf("an account with no log: %v", err)
	}
}
