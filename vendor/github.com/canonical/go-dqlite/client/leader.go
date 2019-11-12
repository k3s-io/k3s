package client

import (
	"context"
	"time"

	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/canonical/go-dqlite/internal/protocol"
)

// FindLeader returns a Client connected to the current cluster leader, if any.
func FindLeader(ctx context.Context, store NodeStore, options ...Option) (*Client, error) {
	o := defaultOptions()

	for _, option := range options {
		option(o)
	}

	config := protocol.Config{
		Dial:           o.DialFunc,
		AttemptTimeout: time.Second,
		RetryStrategies: []strategy.Strategy{
			strategy.Backoff(backoff.BinaryExponential(time.Millisecond))},
	}
	connector := protocol.NewConnector(0, store, config, o.LogFunc)
	protocol, err := connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	client := &Client{protocol: protocol}

	return client, nil
}
