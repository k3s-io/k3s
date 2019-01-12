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

package vxlan

import (
	log "k8s.io/klog"
	"golang.org/x/net/context"
	"sync"

	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/subnet"

	"encoding/json"
	"github.com/Microsoft/hcsshim/hcn"
	"github.com/coreos/flannel/pkg/ip"
	"net"
	"strings"
)

type network struct {
	backend.SimpleNetwork
	dev       *vxlanDevice
	subnetMgr subnet.Manager
}

type vxlanLeaseAttrs struct {
	VNI     uint16
	VtepMAC hardwareAddr
}

const (
	encapOverhead = 50
)

func newNetwork(subnetMgr subnet.Manager, extIface *backend.ExternalInterface, dev *vxlanDevice, _ ip.IP4Net, lease *subnet.Lease) (*network, error) {
	nw := &network{
		SimpleNetwork: backend.SimpleNetwork{
			SubnetLease: lease,
			ExtIface:    extIface,
		},
		subnetMgr: subnetMgr,
		dev:       dev,
	}

	return nw, nil
}

func (nw *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.V(0).Info("Watching for new subnet leases")
	events := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, nw.subnetMgr, nw.SubnetLease, events)
		log.V(1).Info("WatchLeases exited")
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch := <-events:
			nw.handleSubnetEvents(evtBatch)

		case <-ctx.Done():
			return
		}
	}
}

func (nw *network) MTU() int {
	return nw.ExtIface.Iface.MTU - encapOverhead
}

func (nw *network) handleSubnetEvents(batch []subnet.Event) {
	for _, event := range batch {
		leaseSubnet := event.Lease.Subnet
		leaseAttrs := event.Lease.Attrs
		if !strings.EqualFold(leaseAttrs.BackendType, "vxlan") {
			log.Warningf("ignoring non-vxlan subnet(%v): type=%v", leaseSubnet, leaseAttrs.BackendType)
			continue
		}

		var vxlanAttrs vxlanLeaseAttrs
		if err := json.Unmarshal(leaseAttrs.BackendData, &vxlanAttrs); err != nil {
			log.Error("error decoding subnet lease JSON: ", err)
			continue
		}

		hnsnetwork, err := hcn.GetNetworkByName(nw.dev.link.Name)
		if err != nil {
			log.Errorf("Unable to find network %v, error: %v", nw.dev.link.Name, err)
			continue
		}
		managementIp := event.Lease.Attrs.PublicIP.String()

		networkPolicySettings := hcn.RemoteSubnetRoutePolicySetting{
			IsolationId:                 4096,
			DistributedRouterMacAddress: net.HardwareAddr(vxlanAttrs.VtepMAC).String(),
			ProviderAddress:             managementIp,
			DestinationPrefix:           event.Lease.Subnet.String(),
		}
		rawJSON, err := json.Marshal(networkPolicySettings)
		networkPolicy := hcn.NetworkPolicy{
			Type:     hcn.RemoteSubnetRoute,
			Settings: rawJSON,
		}

		policyNetworkRequest := hcn.PolicyNetworkRequest{
			Policies: []hcn.NetworkPolicy{networkPolicy},
		}

		switch event.Type {
		case subnet.EventAdded:
			for _, policy := range hnsnetwork.Policies {
				if policy.Type == hcn.RemoteSubnetRoute {
					existingPolicySettings := hcn.RemoteSubnetRoutePolicySetting{}
					err = json.Unmarshal(policy.Settings, &existingPolicySettings)
					if err != nil {
						log.Error("Failed to unmarshal settings")
					}
					if existingPolicySettings.DestinationPrefix == networkPolicySettings.DestinationPrefix {
						existingJson, err := json.Marshal(existingPolicySettings)
						if err != nil {
							log.Error("Failed to marshal settings")
						}
						existingPolicy := hcn.NetworkPolicy{
							Type:     hcn.RemoteSubnetRoute,
							Settings: existingJson,
						}
						existingPolicyNetworkRequest := hcn.PolicyNetworkRequest{
							Policies: []hcn.NetworkPolicy{existingPolicy},
						}
						hnsnetwork.RemovePolicy(existingPolicyNetworkRequest)
					}
				}
			}
			if networkPolicySettings.DistributedRouterMacAddress != "" {
				hnsnetwork.AddPolicy(policyNetworkRequest)
			}
		case subnet.EventRemoved:
			if networkPolicySettings.DistributedRouterMacAddress != "" {
				hnsnetwork.RemovePolicy(policyNetworkRequest)
			}
		default:
			log.Error("internal error: unknown event type: ", int(event.Type))
		}
	}
}
