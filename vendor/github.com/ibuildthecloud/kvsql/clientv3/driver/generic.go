package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ibuildthecloud/kvsql/pkg/broadcast"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	utiltrace "k8s.io/apiserver/pkg/util/trace"
)

type Generic struct {
	db *sql.DB

	CleanupSQL      string
	GetSQL          string
	ListSQL         string
	ListRevisionSQL string
	ListResumeSQL   string
	ReplaySQL       string
	InsertSQL       string
	GetRevisionSQL  string
	ToDeleteSQL     string
	DeleteOldSQL    string
	revision        int64

	changes     chan *KeyValue
	broadcaster broadcast.Broadcaster
	cancel      func()
}

func (g *Generic) Start(ctx context.Context, db *sql.DB) error {
	g.db = db
	g.changes = make(chan *KeyValue, 1024)

	row := db.QueryRowContext(ctx, g.GetRevisionSQL)
	rev := sql.NullInt64{}
	if err := row.Scan(&rev); err != nil {
		return errors.Wrap(err, "Failed to initialize revision")
	}
	if rev.Int64 == 0 {
		g.revision = 1
	} else {
		g.revision = rev.Int64
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
				_, err := g.ExecContext(ctx, g.CleanupSQL, time.Now().Unix())
				if err != nil {
					logrus.Errorf("Failed to purge expired TTL entries")
				}

				err = g.cleanup(ctx)
				if err != nil {
					logrus.Errorf("Failed to cleanup duplicate entries")
				}
			}
		}
	}()

	return nil
}

func (g *Generic) cleanup(ctx context.Context) error {
	rows, err := g.QueryContext(ctx, g.ToDeleteSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	toDelete := map[string]int64{}
	for rows.Next() {
		var (
			count, revision int64
			name            string
		)
		err := rows.Scan(&count, &name, &revision)
		if err != nil {
			return err
		}
		toDelete[name] = revision
	}

	rows.Close()

	for name, rev := range toDelete {
		_, err = g.ExecContext(ctx, g.DeleteOldSQL, name, rev, rev)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Generic) Get(ctx context.Context, key string) (*KeyValue, error) {
	kvs, _, err := g.List(ctx, 0, 1, key, "")
	if err != nil {
		return nil, err
	}
	if len(kvs) > 0 {
		return kvs[0], nil
	}
	return nil, nil
}

func (g *Generic) replayEvents(ctx context.Context, key string, revision int64) ([]*KeyValue, error) {
	rows, err := g.QueryContext(ctx, g.ReplaySQL, key, revision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resp []*KeyValue
	for rows.Next() {
		value := KeyValue{}
		if err := scan(rows.Scan, &value); err != nil {
			return nil, err
		}
		resp = append(resp, &value)
	}

	return resp, nil
}

func (g *Generic) List(ctx context.Context, revision, limit int64, rangeKey, startKey string) ([]*KeyValue, int64, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if limit == 0 {
		limit = 1000000
	} else {
		limit = limit + 1
	}

	listRevision := atomic.LoadInt64(&g.revision)
	if !strings.HasSuffix(rangeKey, "%") && revision <= 0 {
		rows, err = g.QueryContext(ctx, g.GetSQL, rangeKey, limit)
	} else if revision <= 0 {
		rows, err = g.QueryContext(ctx, g.ListSQL, rangeKey, limit)
	} else if len(startKey) > 0 {
		listRevision = revision
		rows, err = g.QueryContext(ctx, g.ListResumeSQL, revision, rangeKey, startKey, limit)
	} else {
		rows, err = g.QueryContext(ctx, g.ListRevisionSQL, revision, rangeKey, limit)
	}

	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var resp []*KeyValue
	for rows.Next() {
		value := KeyValue{}
		if err := scan(rows.Scan, &value); err != nil {
			return nil, 0, err
		}
		if value.Revision > listRevision {
			listRevision = value.Revision
		}
		if value.Del == 0 {
			resp = append(resp, &value)
		}
	}

	return resp, listRevision, nil
}

func (g *Generic) Delete(ctx context.Context, key string, revision int64) ([]*KeyValue, error) {
	if strings.HasSuffix(key, "%") {
		panic("can not delete list revision")
	}

	_, err := g.mod(ctx, true, key, []byte{}, revision, 0)
	return nil, err
}

func (g *Generic) Update(ctx context.Context, key string, value []byte, revision, ttl int64) (*KeyValue, *KeyValue, error) {
	kv, err := g.mod(ctx, false, key, value, revision, ttl)
	if err != nil {
		return nil, nil, err
	}

	if kv.Version == 1 {
		return nil, kv, nil
	}

	oldKv := *kv
	oldKv.Revision = oldKv.OldRevision
	oldKv.Value = oldKv.OldValue
	return &oldKv, kv, nil
}

func (g *Generic) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	trace := utiltrace.New(fmt.Sprintf("SQL DB ExecContext query: %s keys: %v", query, args))
	defer trace.LogIfLong(500 * time.Millisecond)

	return g.db.ExecContext(ctx, query, args...)
}

func (g *Generic) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	trace := utiltrace.New(fmt.Sprintf("SQL DB QueryContext query: %s keys: %v", query, args))
	defer trace.LogIfLong(500 * time.Millisecond)

	return g.db.QueryContext(ctx, query, args...)
}

func (g *Generic) mod(ctx context.Context, delete bool, key string, value []byte, revision int64, ttl int64) (*KeyValue, error) {
	oldKv, err := g.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if revision > 0 && oldKv == nil {
		return nil, ErrNotExists
	}

	if revision > 0 && oldKv.Revision != revision {
		return nil, ErrRevisionMatch
	}

	if ttl > 0 {
		ttl = int64(time.Now().Unix()) + ttl
	}

	newRevision := atomic.AddInt64(&g.revision, 1)
	result := &KeyValue{
		Key:            key,
		Value:          value,
		Revision:       newRevision,
		TTL:            int64(ttl),
		CreateRevision: newRevision,
		Version:        1,
	}
	if oldKv != nil {
		result.OldRevision = oldKv.Revision
		result.OldValue = oldKv.Value
		result.TTL = oldKv.TTL
		result.CreateRevision = oldKv.CreateRevision
		result.Version = oldKv.Version + 1
	}

	if delete {
		result.Del = 1
	}

	_, err = g.ExecContext(ctx, g.InsertSQL,
		result.Key,
		result.Value,
		result.OldValue,
		result.OldRevision,
		result.CreateRevision,
		result.Revision,
		result.TTL,
		result.Version,
		result.Del,
	)
	if err != nil {
		return nil, err
	}

	g.changes <- result
	return result, nil
}

type scanner func(dest ...interface{}) error

func scan(s scanner, out *KeyValue) error {
	return s(
		&out.ID,
		&out.Key,
		&out.Value,
		&out.OldValue,
		&out.OldRevision,
		&out.CreateRevision,
		&out.Revision,
		&out.TTL,
		&out.Version,
		&out.Del)
}
