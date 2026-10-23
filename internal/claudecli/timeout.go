package claudecli

import (
	"context"
	"time"
)

// authTimeout is how long Status, Logout and Version may take when the caller
// sets no deadline. it is long on purpose: ending claude in the middle of
// a token refresh can sign the folder out, so callers must not use short
// timeouts on auth calls, and this only exists so a hung claude cannot hang
// clalt for ever.
var authTimeout = 2 * time.Minute

// withDefaultTimeout leaves a context that has a deadline alone and gives one
// that has none authTimeout.
func withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, authTimeout)
}
