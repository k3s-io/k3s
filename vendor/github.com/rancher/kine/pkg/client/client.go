package client

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/rancher/kine/pkg/endpoint"
)

type Value struct {
	Data     []byte
	Modified int64
}

type Client interface {
	Get(ctx context.Context, key string) (Value, error)
	Put(ctx context.Context, key string, value []byte) error
	Create(ctx context.Context, key string, value []byte) error
	Update(ctx context.Context, key string, revision int64, value []byte) error
	Close() error
}

type client struct {
	c *clientv3.Client
}

func New(config endpoint.ETCDConfig) (Client, error) {
	tlsConfig, err := config.TLSConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	c, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	return &client{
		c: c,
	}, nil
}

func (c *client) Get(ctx context.Context, key string) (Value, error) {
	resp, err := c.c.Get(ctx, key)
	if err != nil {
		return Value{}, err
	}

	if len(resp.Kvs) == 1 {
		return Value{
			Data:     resp.Kvs[0].Value,
			Modified: resp.Kvs[0].ModRevision,
		}, nil
	}

	return Value{}, nil
}

func (c *client) Put(ctx context.Context, key string, value []byte) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if val.Modified == 0 {
		return c.Create(ctx, key, value)
	}
	return c.Update(ctx, key, val.Modified, value)
}

func (c *client) Create(ctx context.Context, key string, value []byte) error {
	resp, err := c.c.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, string(value))).
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return fmt.Errorf("key exists")
	}
	return nil
}

func (c *client) Update(ctx context.Context, key string, revision int64, value []byte) error {
	resp, err := c.c.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", revision)).
		Then(clientv3.OpPut(key, string(value))).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return fmt.Errorf("revision %d doesnt match", revision)
	}
	return nil
}

func (c *client) Close() error {
	return c.c.Close()
}
