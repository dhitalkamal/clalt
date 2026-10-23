package notify

import (
	"context"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dhitalkamal/clalt/internal/config"
	"github.com/dhitalkamal/clalt/internal/runcmd"
)

// Default is the Alerts of the real machine.
func Default() *Alerts {
	n, _ := System(runcmd.Exec{}, runtime.GOOS)
	return &Alerts{Notifier: n, Dir: filepath.Join(config.ClaltDir(), "notify"), Now: time.Now}
}

// Test sends a sample notification through n.
func Test(ctx context.Context, n Notifier) error {
	return n.Notify(ctx, Note{Title: "clalt", Text: "clalt notifications work"})
}
