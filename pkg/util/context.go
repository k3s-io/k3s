package util

import (
	"context"
	"time"

	"github.com/go-logr/logr"
)

const DefaultContextDelay = 5 * time.Second

// DelayCancel returns a context that will be cancelled
// with a delay after the parent context has been cancelled.
func DelayCancel(ctx context.Context, delay time.Duration) context.Context {
	dctx, dcancel := context.WithCancel(context.Background())
	if l, err := logr.FromContext(ctx); err == nil {
		dctx = logr.NewContext(dctx, l)
	}
	go func() {
		<-ctx.Done()
		time.Sleep(delay)
		dcancel()
	}()
	return dctx
}
