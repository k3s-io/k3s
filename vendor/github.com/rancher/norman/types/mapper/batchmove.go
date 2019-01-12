package mapper

import (
	"path"

	"github.com/rancher/norman/types"
)

type BatchMove struct {
	From              []string
	To                string
	DestDefined       bool
	NoDeleteFromField bool
	moves             []Move
}

func (b *BatchMove) FromInternal(data map[string]interface{}) {
	for _, m := range b.moves {
		m.FromInternal(data)
	}
}

func (b *BatchMove) ToInternal(data map[string]interface{}) error {
	errors := types.Errors{}
	for i := len(b.moves) - 1; i >= 0; i-- {
		errors.Add(b.moves[i].ToInternal(data))
	}
	return errors.Err()
}

func (b *BatchMove) ModifySchema(s *types.Schema, schemas *types.Schemas) error {
	for _, from := range b.From {
		b.moves = append(b.moves, Move{
			From:              from,
			To:                path.Join(b.To, from),
			DestDefined:       b.DestDefined,
			NoDeleteFromField: b.NoDeleteFromField,
		})
	}

	for _, m := range b.moves {
		if err := m.ModifySchema(s, schemas); err != nil {
			return err
		}
	}

	return nil
}
