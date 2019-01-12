package factory

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/mapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Schemas(version *types.APIVersion) *types.Schemas {
	s := types.NewSchemas()
	s.DefaultMappers = func() []types.Mapper {
		return []types.Mapper{
			mapper.NewObject(),
		}
	}
	s.DefaultPostMappers = func() []types.Mapper {
		return []types.Mapper{
			&mapper.RenameReference{},
		}
	}
	s.AddMapperForType(version, v1.ObjectMeta{}, mapper.NewMetadataMapper())
	return s
}
