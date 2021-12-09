// Copyright 2021 flannel authors
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
package wireguard

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/subnet"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

const (
	/*
		20-byte IPv4 header or 40 byte IPv6 header
		8-byte UDP header
		4-byte type
		4-byte key index
		8-byte nonce
		N-byte encrypted data
		16-byte authentication tag
	*/
	overhead = 80
)

type network struct {
	dev      *wgDevice
	v6Dev    *wgDevice
	extIface *backend.ExternalInterface
	lease    *subnet.Lease
	sm       subnet.Manager
}

func newNetwork(sm subnet.Manager, extIface *backend.ExternalInterface, dev, v6Dev *wgDevice, lease *subnet.Lease) (*network, error) {
	n := &network{
		dev:      dev,
		v6Dev:    v6Dev,
		extIface: extIface,
		lease:    lease,
		sm:       sm,
	}

	return n, nil
}

func (n *network) Lease() *subnet.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.extIface.Iface.MTU - overhead
}

func (n *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.Info("Watching for new subnet leases")
	events := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.sm, n.lease, events)
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch := <-events:
			n.handleSubnetEvents(evtBatch)

		case <-ctx.Done():
			return
		}
	}
}

type wireguardLeaseAttrs struct {
	PublicKey string
}

func (n *network) handleSubnetEvents(batch []subnet.Event) {
	for _, event := range batch {
		switch event.Type {
		case subnet.EventAdded:

			if event.Lease.Attrs.BackendType != "wireguard" {
				log.Warningf("Ignoring non-wireguard subnet: type=%v", event.Lease.Attrs.BackendType)
				continue
			}

			var wireguardAttrs wireguardLeaseAttrs
			if event.Lease.EnableIPv4 && n.dev != nil {
				log.Infof("Subnet added: %v via %v", event.Lease.Subnet, event.Lease.Attrs.PublicIP)

				if len(event.Lease.Attrs.BackendData) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendData, &wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %w", err)
						continue
					}
				}

				publicEndpoint := fmt.Sprintf("%s:%d", event.Lease.Attrs.PublicIP.String(), n.dev.attrs.listenPort)
				if err := n.dev.addPeer(
					publicEndpoint,
					wireguardAttrs.PublicKey,
					event.Lease.Subnet.ToIPNet()); err != nil {
					log.Errorf("failed to setup ipv4 peer (%s): %v", wireguardAttrs.PublicKey, err)
				}
			}

			if event.Lease.EnableIPv6 && n.v6Dev != nil {
				log.Infof("Subnet added: %v via %v", event.Lease.IPv6Subnet, event.Lease.Attrs.PublicIPv6)

				if len(event.Lease.Attrs.BackendV6Data) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendV6Data, &wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %w", err)
						continue
					}
				}

				publicEndpoint := fmt.Sprintf("[%s]:%d", event.Lease.Attrs.PublicIPv6.String(), n.v6Dev.attrs.listenPort)
				if err := n.v6Dev.addPeer(
					publicEndpoint,
					wireguardAttrs.PublicKey,
					event.Lease.IPv6Subnet.ToIPNet()); err != nil {
					log.Errorf("failed to setup ipv6 peer (%s): %w", wireguardAttrs.PublicKey, err)
				}
			}

		case subnet.EventRemoved:

			if event.Lease.Attrs.BackendType != "wireguard" {
				log.Warningf("Ignoring non-wireguard subnet: type=%v", event.Lease.Attrs.BackendType)
				continue
			}

			var wireguardAttrs wireguardLeaseAttrs
			if event.Lease.EnableIPv4 && n.dev != nil {
				log.Info("Subnet removed: ", event.Lease.Subnet)
				if len(event.Lease.Attrs.BackendData) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendData, &wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %w", err)
						continue
					}
				}

				if err := n.dev.removePeer(
					wireguardAttrs.PublicKey,
				); err != nil {
					log.Errorf("failed to remove ipv4 peer (%s): %v", wireguardAttrs.PublicKey, err)
				}
			}

			if event.Lease.EnableIPv6 && n.v6Dev != nil {
				log.Info("Subnet removed: ", event.Lease.IPv6Subnet)
				if len(event.Lease.Attrs.BackendV6Data) > 0 {
					if err := json.Unmarshal(event.Lease.Attrs.BackendV6Data, &wireguardAttrs); err != nil {
						log.Errorf("failed to unmarshal BackendData: %w", err)
						continue
					}
				}

				if err := n.v6Dev.removePeer(
					wireguardAttrs.PublicKey,
				); err != nil {
					log.Errorf("failed to remove ipv6 peer (%s): %w", wireguardAttrs.PublicKey, err)
				}
			}

		default:
			log.Error("Internal error: unknown event type: ", int(event.Type))
		}
	}
}
