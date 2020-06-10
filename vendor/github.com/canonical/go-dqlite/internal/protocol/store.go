package protocol

import (
	"context"
)

// NodeRole identifies the role of a node.
type NodeRole int

// String implements the Stringer interface.
func (r NodeRole) String() string {
	switch r {
	case Voter:
		return "voter"
	case StandBy:
		return "stand-by"
	case Spare:
		return "spare"
	default:
		return "unknown role"
	}
}

// NodeInfo holds information about a single server.
type NodeInfo struct {
	ID      uint64
	Address string
	Role    NodeRole
}

// NodeStore is used by a dqlite client to get an initial list of candidate
// dqlite servers that it can dial in order to find a leader server to connect
// to.
//
// Once connected, the client periodically updates the server addresses in the
// store by querying the leader about changes in the cluster (such as servers
// being added or removed).
type NodeStore interface {
	// Get return the list of known servers.
	Get(context.Context) ([]NodeInfo, error)

	// Set updates the list of known cluster servers.
	Set(context.Context, []NodeInfo) error
}

// InmemNodeStore keeps the list of servers in memory.
type InmemNodeStore struct {
	servers []NodeInfo
}

// NewInmemNodeStore creates NodeStore which stores its data in-memory.
func NewInmemNodeStore() *InmemNodeStore {
	return &InmemNodeStore{
		servers: make([]NodeInfo, 0),
	}
}

// Get the current servers.
func (i *InmemNodeStore) Get(ctx context.Context) ([]NodeInfo, error) {
	return i.servers, nil
}

// Set the servers.
func (i *InmemNodeStore) Set(ctx context.Context, servers []NodeInfo) error {
	i.servers = servers
	return nil
}
