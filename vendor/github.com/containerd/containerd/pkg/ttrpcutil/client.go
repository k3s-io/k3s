/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package ttrpcutil

import (
	"sync"
	"time"

	v1 "github.com/containerd/containerd/api/services/ttrpc/events/v1"
	"github.com/containerd/ttrpc"
	"github.com/pkg/errors"
)

const ttrpcDialTimeout = 5 * time.Second

type ttrpcConnector func() (*ttrpc.Client, error)

// Client is the client to interact with TTRPC part of containerd server (plugins, events)
type Client struct {
	mu        sync.Mutex
	connector ttrpcConnector
	client    *ttrpc.Client
	closed    bool
}

// NewClient returns a new containerd TTRPC client that is connected to the containerd instance provided by address
func NewClient(address string, opts ...ttrpc.ClientOpts) (*Client, error) {
	connector := func() (*ttrpc.Client, error) {
		conn, err := ttrpcDial(address, ttrpcDialTimeout)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect")
		}

		client := ttrpc.NewClient(conn, opts...)
		return client, nil
	}

	client, err := connector()
	if err != nil {
		return nil, err
	}

	return &Client{
		connector: connector,
		client:    client,
	}, nil
}

// Reconnect re-establishes the TTRPC connection to the containerd daemon
func (c *Client) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connector == nil {
		return errors.New("unable to reconnect to containerd, no connector available")
	}

	if c.closed {
		return errors.New("client is closed")
	}

	client, err := c.connector()
	if err != nil {
		return err
	}

	c.client = client
	return nil
}

// EventsService creates an EventsService client
func (c *Client) EventsService() v1.EventsService {
	return v1.NewEventsClient(c.Client())
}

// Client returns the underlying TTRPC client object
func (c *Client) Client() *ttrpc.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.client
}

// Close closes the clients TTRPC connection to containerd
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	return c.client.Close()
}
