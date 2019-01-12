package access

import (
	"fmt"

	"github.com/rancher/norman/parse/builder"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func Create(context *types.APIContext, version *types.APIVersion, typeName string, data map[string]interface{}, into interface{}) error {
	schema := context.Schemas.Schema(version, typeName)
	if schema == nil {
		return fmt.Errorf("failed to find schema " + typeName)
	}

	item, err := schema.Store.Create(context, schema, data)
	if err != nil {
		return err
	}

	b := builder.NewBuilder(context)
	b.Version = version

	item, err = b.Construct(schema, item, builder.List)
	if err != nil {
		return err
	}

	if into == nil {
		return nil
	}

	return convert.ToObj(item, into)
}

func ByID(context *types.APIContext, version *types.APIVersion, typeName string, id string, into interface{}) error {
	schema := context.Schemas.Schema(version, typeName)
	if schema == nil {
		return fmt.Errorf("failed to find schema " + typeName)
	}

	item, err := schema.Store.ByID(context, schema, id)
	if err != nil {
		return err
	}

	b := builder.NewBuilder(context)
	b.Version = version

	item, err = b.Construct(schema, item, builder.List)
	if err != nil {
		return err
	}

	if into == nil {
		return nil
	}

	return convert.ToObj(item, into)
}

func List(context *types.APIContext, version *types.APIVersion, typeName string, opts *types.QueryOptions, into interface{}) error {
	schema := context.Schemas.Schema(version, typeName)
	if schema == nil {
		return fmt.Errorf("failed to find schema " + typeName)
	}

	data, err := schema.Store.List(context, schema, opts)
	if err != nil {
		return err
	}

	b := builder.NewBuilder(context)
	b.Version = version

	var newData []map[string]interface{}
	for _, item := range data {
		item, err = b.Construct(schema, item, builder.List)
		if err != nil {
			return err
		}
		newData = append(newData, item)
	}

	return convert.ToObj(newData, into)
}
