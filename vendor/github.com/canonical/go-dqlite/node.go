package dqlite

import (
	"time"

	"github.com/canonical/go-dqlite/client"
	"github.com/canonical/go-dqlite/internal/bindings"
	"github.com/pkg/errors"
)

// Node runs a dqlite node.
type Node struct {
	log         client.LogFunc // Logger
	server      *bindings.Node // Low-level C implementation
	acceptCh    chan error     // Receives connection handling errors
	id          uint64
	address     string
	bindAddress string
}

// NodeInfo is a convenience alias for client.NodeInfo.
type NodeInfo = client.NodeInfo

// Option can be used to tweak node parameters.
type Option func(*options)

// WithDialFunc sets a custom dial function for the server.
func WithDialFunc(dial client.DialFunc) Option {
	return func(options *options) {
		options.DialFunc = dial
	}
}

// WithBindAddress sets a custom bind address for the server.
func WithBindAddress(address string) Option {
	return func(options *options) {
		options.BindAddress = address
	}
}

// WithNetworkLatency sets the average one-way network latency.
func WithNetworkLatency(latency time.Duration) Option {
	return func(options *options) {
		options.NetworkLatency = uint64(latency.Nanoseconds())
	}
}

// New creates a new Node instance.
func New(id uint64, address string, dir string, options ...Option) (*Node, error) {
	o := defaultOptions()

	for _, option := range options {
		option(o)
	}

	server, err := bindings.NewNode(id, address, dir)
	if err != nil {
		return nil, err
	}
	if o.DialFunc != nil {
		if err := server.SetDialFunc(o.DialFunc); err != nil {
			return nil, err
		}
	}
	if o.BindAddress != "" {
		if err := server.SetBindAddress(o.BindAddress); err != nil {
			return nil, err
		}
	}
	if o.NetworkLatency != 0 {
		if err := server.SetNetworkLatency(o.NetworkLatency); err != nil {
			return nil, err
		}
	}
	s := &Node{
		server:      server,
		acceptCh:    make(chan error, 1),
		id:          id,
		address:     address,
		bindAddress: o.BindAddress,
	}

	return s, nil
}

// BindAddress returns the network address the node is listening to.
func (s *Node) BindAddress() string {
	return s.server.GetBindAddress()
}

// Start serving requests.
func (s *Node) Start() error {
	return s.server.Start()
}

// Recover a node by forcing a new cluster configuration.
func (s *Node) Recover(cluster []NodeInfo) error {
	return s.server.Recover(cluster)
}

// Hold configuration options for a dqlite server.
type options struct {
	Log            client.LogFunc
	DialFunc       client.DialFunc
	BindAddress    string
	NetworkLatency uint64
}

// Close the server, releasing all resources it created.
func (s *Node) Close() error {
	// Send a stop signal to the dqlite event loop.
	if err := s.server.Stop(); err != nil {
		return errors.Wrap(err, "server failed to stop")
	}

	s.server.Close()

	return nil
}

// BootstrapID is a magic ID that should be used for the fist node in a
// cluster. Alternatively ID 1 can be used as well.
const BootstrapID = 0x2dc171858c3155be

// GenerateID generates a unique ID for a new node, based on a hash of its
// address and the current time.
func GenerateID(address string) uint64 {
	return bindings.GenerateID(address)
}

// Create a options object with sane defaults.
func defaultOptions() *options {
	return &options{
		DialFunc: client.DefaultDialFunc,
	}
}
