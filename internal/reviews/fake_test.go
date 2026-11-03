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
	// answer, when set, may reply to a call by its content (stdin included)
	// before the rules are tried. ok false falls through to the rules.
	answer func(c call) (out string, ok bool)
	// fail, when set, may make a call return an error by its content (stdin
	// included). it is tried before answer and the rules.
	fail func(c call) error
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
	answer := f.answer
	fail := f.fail
	f.mu.Unlock()
	if hook != nil {
		hook(c)
	}
	if fail != nil {
		if err := fail(c); err != nil {
			return nil, err
		}
	}
	if answer != nil {
		if out, ok := answer(c); ok {
			return []byte(out), nil
		}
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
