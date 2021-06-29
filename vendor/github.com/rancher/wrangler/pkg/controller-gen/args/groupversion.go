package args

import (
	"reflect"
	"strings"

	"k8s.io/code-generator/cmd/client-gen/generators/util"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/code-generator/cmd/client-gen/path"
	"k8s.io/gengo/types"
)

const (
	needsComment = `
		// +genclient
		// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
	`
	objectComment = "+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object"
)

func ObjectsToGroupVersion(group string, objs []interface{}, ret map[schema.GroupVersion][]*types.Name) {
	for _, obj := range objs {
		version, t := toVersionType(obj)
		gv := schema.GroupVersion{
			Group:   group,
			Version: version,
		}
		ret[gv] = append(ret[gv], t)
	}
}

func toVersionType(obj interface{}) (string, *types.Name) {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	pkg := path.Vendorless(t.PkgPath())
	parts := strings.Split(pkg, "/")
	return parts[len(parts)-1], &types.Name{
		Package: pkg,
		Name:    t.Name(),
	}
}

func CheckType(passedType *types.Type) {
	tags := util.MustParseClientGenTags(passedType.SecondClosestCommentLines)
	if !tags.GenerateClient {
		panic("Type " + passedType.String() + " is missing comment " + needsComment)
	}
	found := false
	for _, line := range passedType.SecondClosestCommentLines {
		if strings.Contains(line, objectComment) {
			found = true
		}
	}
	if !found {
		panic("Type " + passedType.String() + " is missing comment " + objectComment)
	}
}
