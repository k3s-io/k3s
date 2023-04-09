//go:build linux && !no_embedded_executor
// +build linux,!no_embedded_executor

package executor

import (
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"

	// registering k3s cloud provider
	_ "github.com/k3s-io/k3s/pkg/cloudprovider"
)

func platformKubeProxyArgs(nodeConfig *daemonconfig.Node) map[string]string {
	argsMap := map[string]string{}
	return argsMap
}
