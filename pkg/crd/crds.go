package crd

import (
	v1 "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	"github.com/rancher/wrangler/v2/pkg/crd"
)

func List() []crd.CRD {
	addon := crd.NamespacedType("Addon.k3s.cattle.io/v1").
		WithSchemaFromStruct(v1.Addon{}).
		WithColumnsFromStruct(v1.Addon{})

	return []crd.CRD{addon}
}
