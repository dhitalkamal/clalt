package runcmd

import (
	"context"
	"strings"
	"sync"
)

// Call is one call made through a Fake.
type Call struct {
	Name string
	Args []string
}

// String shows the call as a command line, for test messages.
func (c Call) String() string { return strings.Join(append([]string{c.Name}, c.Args...), " ") }

// Fake is a Runner that runs nothing. it records every call and answers from the
// rules a test set up with On. a call no rule matches succeeds with no output.
type Fake struct {
	mu      sync.Mutex
	calls   []Call
	rules   []*Rule
	missing map[string]bool
}

// NewFake makes a Fake that has every program and answers every call with
// success.
func NewFake() *Fake { return &Fake{} }

// Rule is how a Fake answers the calls it matches.
type Rule struct {
	name   string
	prefix []string
	out    string
	fail   *Failure
	err    error
}

// On starts a rule for calls of name whose arguments begin with prefix. the
// first rule that matches a call answers it.
func (f *Fake) On(name string, prefix ...string) *Rule {
	r := &Rule{name: name, prefix: append([]string(nil), prefix...)}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules = append(f.rules, r)
	return r
}

// Output makes the rule answer with success and stdout.
func (r *Rule) Output(stdout string) *Rule {
	r.out, r.fail, r.err = stdout, nil, nil
	return r
}

// Fail makes the rule answer with a *Failure, the way a program that exits with
// code does.
func (r *Rule) Fail(code int, stderr string) *Rule {
	r.fail, r.err = &Failure{Name: r.name, Code: code, Stderr: stderr}, nil
	return r
}

// Err makes the rule answer with err, the way a program that cannot run does.
func (r *Rule) Err(err error) *Rule {
	r.err, r.fail = err, nil
	return r
}

func (r *Rule) matches(name string, args []string) bool {
	if r.name != name || len(args) < len(r.prefix) {
		return false
	}
	for i, want := range r.prefix {
		if args[i] != want {
			return false
		}
	}
	return true
}

// Without makes Has report false for names.
func (f *Fake) Without(names ...string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.missing == nil {
		f.missing = map[string]bool{}
	}
	for _, n := range names {
		f.missing[n] = true
	}
	return f
}

// Calls returns every call so far, oldest first.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

// Run implements Runner.
func (f *Fake) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, Call{Name: name, Args: append([]string(nil), args...)})
	for _, r := range f.rules {
		if !r.matches(name, args) {
			continue
		}
		switch {
		case r.err != nil:
			return nil, r.err
		case r.fail != nil:
			fail := *r.fail
			return nil, &fail
		}
		return []byte(r.out), nil
	}
	return nil, nil
}

// Has implements Runner.
func (f *Fake) Has(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.missing[name]
}
