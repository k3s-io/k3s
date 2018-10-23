package driver

import (
	"errors"
)

var (
	ErrExists        = errors.New("key exists")
	ErrNotExists     = errors.New("key and or Revision does not exists")
	ErrRevisionMatch = errors.New("revision does not match")
)

type KeyValue struct {
	ID             int64
	Key            string
	Value          []byte
	OldValue       []byte
	OldRevision    int64
	CreateRevision int64
	Revision       int64
	TTL            int64
	Version        int64
	Del            int64
}
