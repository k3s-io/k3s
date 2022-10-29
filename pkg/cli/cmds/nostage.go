//go:build no_stage
// +build no_stage

package cmds

const (
	// The coredns and servicelb controllers can still be disabled, even if their manifests
	// are missing. Same with CloudController/ccm.
	DisableItems = "coredns, servicelb"
)
