//go:build linux && cover

package cmds

import (
	"context"
	"os"
	"runtime/coverage"
	"time"

	"github.com/sirupsen/logrus"
)

// writeCoverage checks if GOCOVERDIR is set on startup and writes coverage files to that directory
// every 20 seconds. This is done to ensure that the coverage files are written even if the process is killed.
func WriteCoverage(ctx context.Context) {
	if k, ok := os.LookupEnv("GOCOVERDIR"); ok {
		for {
			select {
			case <-ctx.Done():
				if err := coverage.WriteCountersDir(k); err != nil {
					logrus.Warn(err)
				}
				return
			case <-time.After(20 * time.Second):
				if err := coverage.WriteCountersDir(k); err != nil {
					logrus.Warn(err)
				}
			}
		}
	}
}
