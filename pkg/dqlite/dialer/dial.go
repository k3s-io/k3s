package dialer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/canonical/go-dqlite/client"
	"github.com/rancher/k3s/pkg/dqlite/pipe"
)

func NewHTTPDialer(advertiseAddress, bindAddress string, tls *tls.Config) (client.DialFunc, error) {
	d := &dialer{
		advertiseAddress: advertiseAddress,
		bindAddress:      bindAddress,
		tls:              tls,
	}

	return d.dial, nil
}

type dialer struct {
	advertiseAddress string
	bindAddress      string
	tls              *tls.Config
}

func (d *dialer) dial(ctx context.Context, address string) (net.Conn, error) {
	if address == d.advertiseAddress {
		return net.Dial("unix", d.bindAddress)
	}

	url := fmt.Sprintf("https://%s/db/connect", address)
	return pipe.ToHTTP(ctx, url, d.tls)
}
