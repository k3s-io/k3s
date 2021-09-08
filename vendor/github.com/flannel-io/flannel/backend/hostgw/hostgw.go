// +build !windows

// Copyright 2015 flannel authors
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
// +build !windows

package hostgw

import (
	"fmt"
	"sync"

	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
)

func init() {
	backend.Register("host-gw", New)
}

type HostgwBackend struct {
	sm       subnet.Manager
	extIface *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	if !extIface.ExtAddr.Equal(extIface.IfaceAddr) {
		return nil, fmt.Errorf("your PublicIP differs from interface IP, meaning that probably you're on a NAT, which is not supported by host-gw backend")
	}

	be := &HostgwBackend{
		sm:       sm,
		extIface: extIface,
	}
	return be, nil
}

func (be *HostgwBackend) RegisterNetwork(ctx context.Context, wg *sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
	n := &backend.RouteNetwork{
		SimpleNetwork: backend.SimpleNetwork{
			ExtIface: be.extIface,
		},
		SM:          be.sm,
		BackendType: "host-gw",
		Mtu:         be.extIface.Iface.MTU,
		LinkIndex:   be.extIface.Iface.Index,
	}

	attrs := subnet.LeaseAttrs{
		BackendType: "host-gw",
	}

	if config.EnableIPv4 {
		attrs.PublicIP = ip.FromIP(be.extIface.ExtAddr)
		n.GetRoute = func(lease *subnet.Lease) *netlink.Route {
			return &netlink.Route{
				Dst:       lease.Subnet.ToIPNet(),
				Gw:        lease.Attrs.PublicIP.ToIP(),
				LinkIndex: n.LinkIndex,
			}
		}
	}

	if config.EnableIPv6 {
		attrs.PublicIPv6 = ip.FromIP6(be.extIface.ExtV6Addr)
		n.GetV6Route = func(lease *subnet.Lease) *netlink.Route {
			return &netlink.Route{
				Dst:       lease.IPv6Subnet.ToIPNet(),
				Gw:        lease.Attrs.PublicIPv6.ToIP(),
				LinkIndex: n.LinkIndex,
			}
		}
	}

	l, err := be.sm.AcquireLease(ctx, &attrs)
	switch err {
	case nil:
		n.SubnetLease = l

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	return n, nil
}
