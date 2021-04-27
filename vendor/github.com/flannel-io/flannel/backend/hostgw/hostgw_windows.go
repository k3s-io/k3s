// Copyright 2018 flannel authors
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

package hostgw

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/routing"
	"github.com/flannel-io/flannel/subnet"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	log "k8s.io/klog"
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
	// 1. Parse configuration
	cfg := struct {
		Name          string
		DNSServerList string
	}{}
	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, errors.Wrap(err, "error decoding windows host-gw backend config")
		}
	}
	if len(cfg.Name) == 0 {
		cfg.Name = "cbr0"
	}
	log.Infof("HOST-GW config: %+v", cfg)

	n := &backend.RouteNetwork{
		SimpleNetwork: backend.SimpleNetwork{
			ExtIface: be.extIface,
		},
		SM:          be.sm,
		BackendType: "host-gw",
		Mtu:         be.extIface.Iface.MTU,
		LinkIndex:   be.extIface.Iface.Index,
	}
	n.GetRoute = func(lease *subnet.Lease) *routing.Route {
		return &routing.Route{
			DestinationSubnet: lease.Subnet.ToIPNet(),
			GatewayAddress:    lease.Attrs.PublicIP.ToIP(),
			InterfaceIndex:    n.LinkIndex,
		}
	}

	// 2. Acquire the lease form subnet manager
	attrs := subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(be.extIface.ExtAddr),
		BackendType: "host-gw",
	}

	l, err := be.sm.AcquireLease(ctx, &attrs)
	switch err {
	case nil:
		n.SubnetLease = l

	case context.Canceled, context.DeadlineExceeded:
		return nil, err

	default:
		return nil, errors.Wrap(err, "failed to acquire lease")
	}

	// 3. Check if the network exists and has the expected settings
	createNewNetwork := true
	expectedSubnet := n.SubnetLease.Subnet
	expectedAddressPrefix := expectedSubnet.String()
	expectedGatewayAddress := (expectedSubnet.IP + 1).String()
	expectedPodGatewayAddress := expectedSubnet.IP + 2
	networkName := cfg.Name
	var waitErr, lastErr error

	existingNetwork, err := hcsshim.GetHNSNetworkByName(networkName)
	if err == nil {
		for _, subnet := range existingNetwork.Subnets {
			if subnet.AddressPrefix == expectedAddressPrefix && subnet.GatewayAddress == expectedGatewayAddress {
				createNewNetwork = false
				log.Infof("Found existing HNSNetwork %s", networkName)
				break
			}
		}
	}

	// 4. Create a new HNSNetwork
	expectedNetwork := existingNetwork
	if createNewNetwork {
		if existingNetwork != nil {
			if _, err := existingNetwork.Delete(); err != nil {
				return nil, errors.Wrapf(err, "failed to delete existing HNSNetwork %s", networkName)
			}
			log.Infof("Deleted stale HNSNetwork %s", networkName)
		}

		expectedNetwork = &hcsshim.HNSNetwork{
			Name:          networkName,
			Type:          "L2Bridge",
			DNSServerList: cfg.DNSServerList,
			Subnets: []hcsshim.Subnet{
				{
					AddressPrefix:  expectedAddressPrefix,
					GatewayAddress: expectedGatewayAddress,
				},
			},
		}
		jsonRequest, err := json.Marshal(expectedNetwork)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal %+v", expectedNetwork)
		}

		log.Infof("Attempting to create HNSNetwork %s", string(jsonRequest))
		newNetwork, err := hcsshim.HNSNetworkRequest("POST", "", string(jsonRequest))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create HNSNetwork %s", networkName)
		}

		// Wait for the network to populate Management IP
		log.Infof("Waiting to get ManagementIP from HNSNetwork %s", networkName)
		var newNetworkID = newNetwork.Id
		waitErr = wait.Poll(500*time.Millisecond, 30*time.Second, func() (done bool, err error) {
			newNetwork, lastErr = hcsshim.HNSNetworkRequest("GET", newNetworkID, "")
			return newNetwork != nil && len(newNetwork.ManagementIP) != 0, nil
		})
		if waitErr == wait.ErrWaitTimeout {
			// Do not swallow the root cause
			if lastErr != nil {
				waitErr = lastErr
			}
			return nil, errors.Wrapf(waitErr, "timeout, failed to get management IP from HNSNetwork %s", networkName)
		}

		// Wait for the interface with the management IP
		log.Infof("Waiting to get net interface for HNSNetwork %s (%s)", networkName, newNetwork.ManagementIP)
		managementIP, err := ip.ParseIP4(newNetwork.ManagementIP)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse management ip (%s)", newNetwork.ManagementIP)
		}

		waitErr = wait.Poll(500*time.Millisecond, 5*time.Second, func() (done bool, err error) {
			_, lastErr = ip.GetInterfaceByIP(managementIP.ToIP())
			return lastErr == nil, nil
		})
		if waitErr == wait.ErrWaitTimeout {
			return nil, errors.Wrapf(lastErr, "timeout, failed to get net interface for HNSNetwork %s (%s)", networkName, newNetwork.ManagementIP)
		}

		log.Infof("Created HNSNetwork %s", networkName)
		expectedNetwork = newNetwork
	}

	// 5. Ensure a 1.2 endpoint on this network in the host compartment
	createNewBridgeEndpoint := true
	bridgeEndpointName := networkName + "_ep"
	existingBridgeEndpoint, err := hcsshim.GetHNSEndpointByName(bridgeEndpointName)
	if err == nil && existingBridgeEndpoint.IPAddress.String() == expectedPodGatewayAddress.String() {
		log.Infof("Found existing bridge HNSEndpoint %s", bridgeEndpointName)
		createNewBridgeEndpoint = false
	}

	// 6. Create a bridge HNSEndpoint
	expectedBridgeEndpoint := existingBridgeEndpoint
	if createNewBridgeEndpoint {
		if existingBridgeEndpoint != nil {
			if _, err = existingBridgeEndpoint.Delete(); err != nil {
				return nil, errors.Wrapf(err, "failed to delete existing bridge HNSEndpoint %s", bridgeEndpointName)
			}
			log.Infof("Deleted stale bridge HNSEndpoint %s", bridgeEndpointName)
		}

		expectedBridgeEndpoint = &hcsshim.HNSEndpoint{
			Name:           bridgeEndpointName,
			IPAddress:      expectedPodGatewayAddress.ToIP(),
			VirtualNetwork: expectedNetwork.Id,
		}

		log.Infof("Attempting to create bridge HNSEndpoint %+v", expectedBridgeEndpoint)
		if expectedBridgeEndpoint, err = expectedBridgeEndpoint.Create(); err != nil {
			return nil, errors.Wrapf(err, "failed to create bridge HNSEndpoint %s", bridgeEndpointName)
		}

		log.Infof("Created bridge HNSEndpoint %s", bridgeEndpointName)
	}

	// Wait for the bridgeEndpoint to attach to the host
	log.Infof("Waiting to attach bridge endpoint %s to host", bridgeEndpointName)
	waitErr = wait.Poll(500*time.Millisecond, 5*time.Second, func() (done bool, err error) {
		lastErr = expectedBridgeEndpoint.HostAttach(1)
		if lastErr == nil {
			return true, nil
		}
		// See https://github.com/flannel-io/flannel/issues/1391 and
		// hcsshim lacks some validations to detect the error, so we judge it by error message.
		if strings.Contains(lastErr.Error(), "This endpoint is already attached to the switch.") {
			return true, nil
		}
		return false, nil
	})
	if waitErr == wait.ErrWaitTimeout {
		return nil, errors.Wrapf(lastErr, "failed to hot attach bridge HNSEndpoint %s to host compartment", bridgeEndpointName)
	}
	log.Infof("Attached bridge endpoint %s to host successfully", bridgeEndpointName)

	// 7. Enable forwarding on the host interface and endpoint
	for _, interfaceIpAddress := range []string{expectedNetwork.ManagementIP, expectedBridgeEndpoint.IPAddress.String()} {
		ipv4, err := ip.ParseIP4(interfaceIpAddress)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse expected net interface ip (%s)", interfaceIpAddress)
		}

		netInterface, err := ip.GetInterfaceByIP(ipv4.ToIP())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find interface for IP Address %s", interfaceIpAddress)
		}
		log.Infof("Found %+v interface with IP %s", netInterface, interfaceIpAddress)

		// When a new hns network is created, the interface is modified, esp the name, index
		if expectedNetwork.ManagementIP == ipv4.String() {
			n.LinkIndex = netInterface.Index
			n.Name = netInterface.Name
		}

		if err := ip.EnableForwardingForInterface(netInterface); err != nil {
			return nil, errors.Wrapf(err, "failed to enable forwarding on %s index %d", netInterface.Name, netInterface.Index)
		}
		log.Infof("Enabled forwarding on %s index %d", netInterface.Name, netInterface.Index)
	}

	return n, nil
}
