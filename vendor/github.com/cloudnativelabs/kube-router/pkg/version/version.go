package version

import (
	"fmt"
	"os"
	"runtime"

	"k8s.io/klog/v2"
)

// Version and BuildDate are injected at build time via ldflags
var Version string
var BuildDate string

func PrintVersion(logOutput bool) {
	output := fmt.Sprintf("Running %v version %s, built on %s, %s\n", os.Args[0], Version, BuildDate, runtime.Version())

	if !logOutput {
		_, _ = fmt.Fprintf(os.Stderr, "%s", output)
	} else {
		klog.Info(output)
	}
}
