package reviews

import (
	"context"
	"strings"
	"sync"
)

// call is one program run seen by a fakeExec.
type call struct {
	Dir   string
	Env   []string
	Stdin string
	Name  string
	Args  []string
}

func (c call) line() string { return strings.Join(append([]string{c.Name}, c.Args...), " ") }

// fakeExec records calls and answers from the first rule whose text is
// contained in the command line. it never runs anything.
type fakeExec struct {
	mu    sync.Mutex
	calls []call
	rules []fakeRule
	hook  func(c call)
}

type fakeRule struct {
	has string
	out string
	err error
}

func (f *fakeExec) on(has, out string, err error) *fakeExec {
	f.rules = append(f.rules, fakeRule{has, out, err})
	return f
}

func (f *fakeExec) Run(ctx context.Context, dir string, env []string, stdin string, name string, args ...string) ([]byte, error) {
	c := call{Dir: dir, Env: env, Stdin: stdin, Name: name, Args: args}
	f.mu.Lock()
	f.calls = append(f.calls, c)
	hook := f.hook
	f.mu.Unlock()
	if hook != nil {
		hook(c)
	}
	for _, r := range f.rules {
		if strings.Contains(c.line(), r.has) {
			return []byte(r.out), r.err
		}
	}
	return nil, nil
}

func (f *fakeExec) lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, c.line())
	}
	return out
}
