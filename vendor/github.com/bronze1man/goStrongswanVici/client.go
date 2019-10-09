package goStrongswanVici

import (
	"net"
)

type ClientOptions struct {
	Network string
	Addr    string
	// Dialer creates new network connection and has priority over
	// Network and Addr options.
	Dialer func() (net.Conn, error)
}

type Client struct {
	o ClientOptions
}

func NewClient(options ClientOptions) (client *Client) {
	if options.Dialer == nil {
		options.Dialer = func() (net.Conn, error) {
			return net.Dial(options.Network, options.Addr)
		}
	}
	return &Client{
		o: options,
	}
}

func NewClientFromDefaultSocket() (client *Client) {
	return NewClient(ClientOptions{
		Network: "unix",
		Addr:    "/var/run/charon.vici",
	})
}

func (c *Client) NewConn() (conn *ClientConn, err error) {
	conn1, err := c.o.Dialer()
	if err != nil {
		return nil, err
	}
	return NewClientConn(conn1), nil
}

func (c *Client) ListSas(ike string, ike_id string) (sas []map[string]IkeSa, err error) {
	conn, err := c.NewConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.ListSas(ike, ike_id)
}

func (c *Client) ListAllVpnConnInfo() (list []VpnConnInfo, err error) {
	conn, err := c.NewConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.ListAllVpnConnInfo()
}

func (c *Client) Version() (out *Version, err error) {
	conn, err := c.NewConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.Version()
}

func (c *Client) Terminate(r *TerminateRequest) (err error) {
	conn, err := c.NewConn()
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Terminate(r)
}
