package mapper

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type UnionMapping struct {
	FieldName   string
	CheckFields []string
}

type UnionEmbed struct {
	Fields []UnionMapping
	embeds map[string]Embed
}

func (u *UnionEmbed) FromInternal(data map[string]interface{}) {
	for _, embed := range u.embeds {
		embed.FromInternal(data)
	}
}

func (u *UnionEmbed) ToInternal(data map[string]interface{}) error {
outer:
	for _, mapper := range u.Fields {
		if len(mapper.CheckFields) == 0 {
			continue
		}

		for _, check := range mapper.CheckFields {
			v, ok := data[check]
			if !ok || convert.IsAPIObjectEmpty(v) {
				continue outer
			}
		}

		embed := u.embeds[mapper.FieldName]
		return embed.ToInternal(data)
	}

	return nil
}

func (u *UnionEmbed) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	u.embeds = map[string]Embed{}

	for _, mapping := range u.Fields {
		embed := Embed{
			Field:          mapping.FieldName,
			ignoreOverride: true,
		}
		if err := embed.ModifySchema(schema, schemas); err != nil {
			return err
		}

		for _, checkField := range mapping.CheckFields {
			if _, ok := schema.ResourceFields[checkField]; !ok {
				return fmt.Errorf("missing check field %s on schema %s", checkField, schema.ID)
			}
		}

		u.embeds[mapping.FieldName] = embed
	}

	return nil
}
