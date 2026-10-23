package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// until a test installs the fake, kirocrew is a path that does not exist, so a
// test that forgot cannot reach the real one.
func init() { os.Setenv("CLALT_KIROCREW_BIN", "/nonexistent/clalt-test-kirocrew") }

// fakeKirocrewScript stands in for kirocrew. a gateway is a process that
// records how it was started, prints a line with a token in it, and holds until
// a release file appears (or a minute has passed).
const fakeKirocrewScript = `#!/bin/sh
dir=$(dirname "$0")
case "$1" in
  --version) echo "kirocrew ${FAKE_KIROCREW_VERSION:-0.9.0}"; exit 0 ;;
  config) echo "config $*" >> "$dir/calls"; exit 0 ;;
  gateway)
    echo "gateway $*" >> "$dir/calls"
    env | sort > "$dir/gateway.env"
    echo "gateway up, open http://localhost/?token=abc123secret"
    i=0
    while [ ! -f "$dir/release" ] && [ $i -lt 1200 ]; do sleep 0.05; i=$((i+1)); done
    exit 0 ;;
esac
echo "fake kirocrew: unexpected $*" >&2
exit 64
`

type fakeKirocrew struct {
	t   *testing.T
	dir string
}

// installFakeKirocrew makes the fake the kirocrew of the test and lets all
// gateways go when the test ends.
func installFakeKirocrew(t *testing.T) *fakeKirocrew {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("these tests use sh scripts as kirocrew")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "kirocrew")
	if err := os.WriteFile(bin, []byte(fakeKirocrewScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLALT_KIROCREW_BIN", bin)
	t.Setenv("FAKE_KIROCREW_VERSION", "")
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "release"), nil, 0o644)
	})
	return &fakeKirocrew{t: t, dir: dir}
}

func (f *fakeKirocrew) calls() []string {
	b, _ := os.ReadFile(filepath.Join(f.dir, "calls"))
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func (f *fakeKirocrew) gatewayEnv() map[string]string {
	// the fake is detached and Ready is stubbed, so the file can still be
	// empty or half written when start returns: wait for it to fill
	var b []byte
	var err error
	for i := 0; i < 100; i++ {
		b, err = os.ReadFile(filepath.Join(f.dir, "gateway.env"))
		if err == nil && strings.Contains(string(b), "KIROCREW_PORT=") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		f.t.Fatalf("no gateway was started: %v", err)
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return env
}

// crewEnv is a clalt install with a signed-in account called work and the fake kirocrew.
func crewEnv(t *testing.T) (*testEnv, *fakeKirocrew) {
	t.Helper()
	e := newEnv(t)
	e.seedShared()
	k := installFakeKirocrew(t)
	// every gateway looks ready at once, and the fake has no sockets to inspect
	e.app.crewOpts.Ready = func(context.Context, int) bool { return true }
	e.app.crewOpts.Listeners = func(context.Context, int) ([]string, error) { return []string{"127.0.0.1:5500"}, nil }
	e.signIn("work", "w@example.com")
	return e, k
}

// stopFake lets every gateway of the fake end by itself.
func stopFake(t *testing.T, k *fakeKirocrew) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(k.dir, "release"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}
