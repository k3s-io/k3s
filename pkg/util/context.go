package util

import (
	"context"
	"time"

	"k8s.io/klog/v2"
)

const DefaultContextDelay = 5 * time.Second

// DelayCancel returns a context that will be cancelled
// with a delay after the parent context has been cancelled.
func DelayCancel(ctx context.Context, delay time.Duration) context.Context {
	dctx, dcancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		time.Sleep(delay)
		dcancel()
	}()
	return klog.NewContext(dctx, klog.FromContext(ctx))
}
