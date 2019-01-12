package mapper

import (
	"github.com/rancher/norman/types"
)

type Object struct {
	types.Mappers
}

func NewObject(mappers ...types.Mapper) Object {
	return Object{
		Mappers: append([]types.Mapper{
			&APIGroup{},
			&Embed{Field: "metadata"},
			&Embed{Field: "spec", Optional: true},
			&ReadOnly{Field: "status", Optional: true, SubFields: true},
			Drop{Field: "kind"},
			Drop{Field: "apiVersion"},
			Move{From: "selfLink", To: ".selfLink", DestDefined: true},
			&Scope{
				IfNot: types.NamespaceScope,
				Mappers: []types.Mapper{
					&Drop{Field: "namespace"},
				},
			},
			Drop{Field: "finalizers", IgnoreDefinition: true},
		}, mappers...),
	}
}
