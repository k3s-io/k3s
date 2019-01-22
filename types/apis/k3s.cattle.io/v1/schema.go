package v1

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/factory"
)

var (
	APIVersion = types.APIVersion{
		Version: "v1",
		Group:   "k3s.cattle.io",
		Path:    "/v1-k3s",
	}

	Schemas = factory.Schemas(&APIVersion).
		MustImport(&APIVersion, ListenerConfig{}).
		MustImport(&APIVersion, Addon{})
)
