package crd

import (
	v1 "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/crd"
)

func List() []crd.CRD {
	addon := crd.NamespacedType("Addon.k3s.cattle.io/v1").
		WithSchemaFromStruct(v1.Addon{}).
		WithColumn("Source", ".spec.source").
		WithColumn("Checksum", ".spec.checksum")

	return []crd.CRD{addon}
}
