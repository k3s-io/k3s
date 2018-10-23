// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clientv3

import (
	"google.golang.org/grpc"
)

// Client provides and manages an etcd v3 client session.
type Client struct {
	Cluster
	KV
	Lease
	Watcher
	callOpts []grpc.CallOption
}

// New creates a new etcdv3 client from a given configuration.
func New(cfg Config) (*Client, error) {
	c := &Client{
		Lease: &lessor{},
	}
	kv, err := newKV(cfg)
	if err != nil {
		return nil, err
	}
	c.KV = kv
	c.Watcher = kv
	return c, nil
}

// Close shuts down the client's etcd connections.
func (c *Client) Close() error {
	if c.Watcher != nil {
		return c.Watcher.Close()
	}
	return nil
}

// Endpoints lists the registered endpoints for the client.
func (c *Client) Endpoints() (eps []string) {
	return
}
