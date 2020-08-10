package stats

import (
	// go mod will not vendor without an import for metrics.proto
	_ "github.com/containerd/cgroups/stats/v1"
)
