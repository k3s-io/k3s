package generator

import (
	"fmt"
	"path"
	"strings"

	"github.com/rancher/norman/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	baseCattle = "client"
	baseK8s    = "apis"
)

func DefaultGenerate(schemas *types.Schemas, pkgPath string, publicAPI bool, privateTypes map[string]bool) error {
	version := getVersion(schemas)
	group := strings.Split(version.Group, ".")[0]

	cattleOutputPackage := path.Join(pkgPath, baseCattle, group, version.Version)
	if !publicAPI {
		cattleOutputPackage = ""
	}
	k8sOutputPackage := path.Join(pkgPath, baseK8s, version.Group, version.Version)

	if err := Generate(schemas, privateTypes, cattleOutputPackage, k8sOutputPackage); err != nil {
		return err
	}

	return nil
}

func ControllersForForeignTypes(baseOutputPackage string, gv schema.GroupVersion, nsObjs []interface{}, objs []interface{}) error {
	version := gv.Version
	group := gv.Group
	groupPath := group

	if groupPath == "" {
		groupPath = "core"
	}

	k8sOutputPackage := path.Join(baseOutputPackage, baseK8s, groupPath, version)

	return GenerateControllerForTypes(&types.APIVersion{
		Version: version,
		Group:   group,
		Path:    fmt.Sprintf("/k8s/%s-%s", groupPath, version),
	}, k8sOutputPackage, nsObjs, objs)
}

func getVersion(schemas *types.Schemas) *types.APIVersion {
	var version types.APIVersion
	for _, schema := range schemas.Schemas() {
		if version.Group == "" {
			version = schema.Version
			continue
		}
		if version.Group != schema.Version.Group ||
			version.Version != schema.Version.Version {
			panic("schema set contains two APIVersions")
		}
	}

	return &version
}
