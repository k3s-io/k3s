//go:build !no_stage

package cmds

const (
	// coredns and servicelb run controllers that are turned off when their manifests are disabled.
	// The k3s CloudController also has a bundled manifest and can be disabled via the
	// --disable-cloud-controller flag or --disable=ccm, but the latter method is not documented.
	DisableItems = "coredns, servicelb, traefik, local-storage, metrics-server"
)
