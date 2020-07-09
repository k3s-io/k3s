package version

import "strings"

var (
	Program      = "k3s"
	ProgramUpper = strings.ToUpper(Program)
	Version      = "dev"
	GitCommit    = "HEAD"
)
