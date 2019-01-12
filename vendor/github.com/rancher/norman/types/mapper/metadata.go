package mapper

import (
	"github.com/rancher/norman/types"
)

func NewMetadataMapper() types.Mapper {
	return types.Mappers{
		ChangeType{Field: "name", Type: "dnsLabel"},
		Drop{Field: "generateName"},
		Move{From: "uid", To: "uuid", CodeName: "UUID"},
		Drop{Field: "resourceVersion"},
		Drop{Field: "generation"},
		Move{From: "creationTimestamp", To: "created"},
		Move{From: "deletionTimestamp", To: "removed"},
		Drop{Field: "deletionGracePeriodSeconds"},
		Drop{Field: "initializers"},
		Drop{Field: "clusterName"},
		ReadOnly{Field: "*"},
		Access{
			Fields: map[string]string{
				"name":        "c",
				"namespace":   "c",
				"labels":      "cu",
				"annotations": "cu",
			},
		},
	}
}
