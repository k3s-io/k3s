//go:build windows
// +build windows

package config

import (
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/pkg/errors"
	"github.com/rancher/permissions/pkg/access"
	"github.com/rancher/permissions/pkg/acl"
	"github.com/rancher/permissions/pkg/sid"
	"golang.org/x/sys/windows"
)

func applyContainerdStateAndAddress(nodeConfig *config.Node) {
	nodeConfig.Containerd.State = filepath.Join(nodeConfig.Containerd.Root, "state")
	nodeConfig.Containerd.Address = "npipe:////./pipe/containerd-containerd"
}

func applyCRIDockerdAddress(nodeConfig *config.Node) {
	nodeConfig.CRIDockerd.Address = "npipe:////.pipe/cri-dockerd"
}

func applyContainerdQoSClassConfigFileIfPresent(envInfo *cmds.Agent, containerdConfig *config.Containerd) {
	// QoS-class resource management not supported on windows.
}

// configureACL will configure an Access Control List for the specified file,
// ensuring that only the LocalSystem and Administrators Group have access to the file contents
func configureACL(file string) error {
	// by default Apply will use the current user (LocalSystem in the case of a Windows service)
	// as the owner and current user group as the allowed group
	// additionally, we define a DACL to permit access to the file to the local system and all administrators
	if err := acl.Apply(file, nil, nil, []windows.EXPLICIT_ACCESS{
		access.GrantSid(windows.GENERIC_ALL, sid.LocalSystem()),
		access.GrantSid(windows.GENERIC_ALL, sid.BuiltinAdministrators()),
	}...); err != nil {
		return errors.Wrapf(err, "failed to configure Access Control List For %s", file)
	}

	return nil
}
