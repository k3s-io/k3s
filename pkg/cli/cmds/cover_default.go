//go:build !linux || !cover

package cmds

import "context"

func WriteCoverage(ctx context.Context) {}
