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

package vxlan

import (
	"encoding/json"
	"fmt"
	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/hcn"
	"github.com/buger/jsonparser"
	"github.com/coreos/flannel/pkg/ip"
	log "k8s.io/klog"
	"github.com/juju/errors"
	"github.com/rakelkar/gonetsh/netsh"
	"k8s.io/apimachinery/pkg/util/wait"
	utilexec "k8s.io/utils/exec"
	"time"
)

type vxlanDeviceAttrs struct {
	vni           uint32
	name          string
	gbp           bool
	addressPrefix ip.IP4Net
}

type vxlanDevice struct {
	link          *hcsshim.HNSNetwork
	macPrefix     string
	directRouting bool
}

func newVXLANDevice(devAttrs *vxlanDeviceAttrs) (*vxlanDevice, error) {
	hnsNetwork := &hcsshim.HNSNetwork{
		Name:    devAttrs.name,
		Type:    "Overlay",
		Subnets: make([]hcsshim.Subnet, 0, 1),
	}

	hnsNetwork, err := ensureNetwork(hnsNetwork, int64(devAttrs.vni), devAttrs.addressPrefix.String(), (devAttrs.addressPrefix.IP + 1).String())
	if err != nil {
		return nil, err
	}

	return &vxlanDevice{
		link: hnsNetwork,
	}, nil
}

func ensureNetwork(expectedNetwork *hcsshim.HNSNetwork, expectedVSID int64, expectedAddressPrefix, expectedGW string) (*hcsshim.HNSNetwork, error) {
	createNetwork := true
	networkName := expectedNetwork.Name

	// 1. Check if the HNSNetwork exists and has the expected settings
	existingNetwork, err := hcsshim.GetHNSNetworkByName(networkName)
	if err == nil {
		if existingNetwork.Type == expectedNetwork.Type {
			for _, existingSubnet := range existingNetwork.Subnets {
				if existingSubnet.AddressPrefix == expectedAddressPrefix && existingSubnet.GatewayAddress == expectedGW {
					createNetwork = false
					log.Infof("Found existing HNSNetwork %s", networkName)
					break
				}
			}
		}
	}

	// 2. Create a new HNSNetwork
	if createNetwork {
		if existingNetwork != nil {
			if _, err := existingNetwork.Delete(); err != nil {
				return nil, errors.Annotatef(err, "failed to delete existing HNSNetwork %s", networkName)
			}
			log.Infof("Deleted stale HNSNetwork %s", networkName)
		}

		// Add a VxLan subnet
		expectedNetwork.Subnets = append(expectedNetwork.Subnets, hcsshim.Subnet{
			AddressPrefix:  expectedAddressPrefix,
			GatewayAddress: expectedGW,
			Policies: []json.RawMessage{
				[]byte(fmt.Sprintf(`{"Type":"VSID","VSID":%d}`, expectedVSID)),
			},
		})

		// Config request params
		jsonRequest, err := json.Marshal(expectedNetwork)
		if err != nil {
			return nil, errors.Annotatef(err, "failed to marshal %+v", expectedNetwork)
		}

		log.Infof("Attempting to create HNSNetwork %s", string(jsonRequest))
		newNetwork, err := hcsshim.HNSNetworkRequest("POST", "", string(jsonRequest))
		if err != nil {
			return nil, errors.Annotatef(err, "failed to create HNSNetwork %s", networkName)
		}

		var waitErr, lastErr error
		// Wait for the network to populate Management IP
		log.Infof("Waiting to get ManagementIP from HNSNetwork %s", networkName)
		waitErr = wait.Poll(500*time.Millisecond, 5*time.Second, func() (done bool, err error) {
			newNetwork, lastErr = hcsshim.HNSNetworkRequest("GET", newNetwork.Id, "")
			return newNetwork != nil && len(newNetwork.ManagementIP) != 0, nil
		})
		if waitErr == wait.ErrWaitTimeout {
			return nil, errors.Annotatef(lastErr, "timeout, failed to get management IP from HNSNetwork %s", networkName)
		}

		// Wait for the interface with the management IP
		netshHelper := netsh.New(utilexec.New())
		log.Infof("Waiting to get net interface for HNSNetwork %s (%s)", networkName, newNetwork.ManagementIP)
		waitErr = wait.Poll(500*time.Millisecond, 5*time.Second, func() (done bool, err error) {
			_, lastErr = netshHelper.GetInterfaceByIP(newNetwork.ManagementIP)
			return lastErr == nil, nil
		})
		if waitErr == wait.ErrWaitTimeout {
			return nil, errors.Annotatef(lastErr, "timeout, failed to get net interface for HNSNetwork %s (%s)", networkName, newNetwork.ManagementIP)
		}

		log.Infof("Created HNSNetwork %s", networkName)
		existingNetwork = newNetwork
	}

	existingNetworkV2, err := hcn.GetNetworkByID(existingNetwork.Id)
	if err != nil {
		return nil, errors.Annotatef(err, "Could not find vxlan0 in V2")
	}

	addHostRoute := true
	for _, policy := range existingNetworkV2.Policies {
		if policy.Type == hcn.HostRoute {
			addHostRoute = false
		}
	}
	if addHostRoute {
		hostRoutePolicy := hcn.NetworkPolicy{
			Type:     hcn.HostRoute,
			Settings: []byte("{}"),
		}

		networkRequest := hcn.PolicyNetworkRequest{
			Policies: []hcn.NetworkPolicy{hostRoutePolicy},
		}
		existingNetworkV2.AddPolicy(networkRequest)
	}

	return existingNetwork, nil
}

type neighbor struct {
	MAC               string
	IP                ip.IP4
	ManagementAddress string
}

func (dev *vxlanDevice) AddEndpoint(n *neighbor) error {
	endpointName := createEndpointName(n.IP)

	// 1. Check if the HNSEndpoint exists and has the expected settings
	existingEndpoint, err := hcsshim.GetHNSEndpointByName(endpointName)
	if err == nil && existingEndpoint.VirtualNetwork == dev.link.Id {
		// Check policies if there is PA type
		targetType := "PA"
		for _, policy := range existingEndpoint.Policies {
			policyType, _ := jsonparser.GetUnsafeString(policy, "Type")
			if policyType == targetType {
				actualPaIP, _ := jsonparser.GetUnsafeString(policy, targetType)
				if actualPaIP == n.ManagementAddress {
					log.Infof("Found existing remote HNSEndpoint %s", endpointName)
					return nil
				}
			}
		}
	}

	// 2. Create a new HNSNetwork
	if existingEndpoint != nil {
		if _, err := existingEndpoint.Delete(); err != nil {
			return errors.Annotatef(err, "failed to delete existing remote HNSEndpoint %s", endpointName)
		}
		log.V(4).Infof("Deleted stale HNSEndpoint %s", endpointName)
	}

	newEndpoint := &hcsshim.HNSEndpoint{
		Name:             endpointName,
		IPAddress:        n.IP.ToIP(),
		MacAddress:       n.MAC,
		VirtualNetwork:   dev.link.Id,
		IsRemoteEndpoint: true,
		Policies: []json.RawMessage{
			[]byte(fmt.Sprintf(`{"Type":"PA","PA":"%s"}`, n.ManagementAddress)),
		},
	}
	if _, err := newEndpoint.Create(); err != nil {
		return errors.Annotatef(err, "failed to create remote HNSEndpoint %s", endpointName)
	}
	log.V(4).Infof("Created HNSEndpoint %s", endpointName)

	return nil
}

func (dev *vxlanDevice) DelEndpoint(n *neighbor) error {
	endpointName := createEndpointName(n.IP)

	existingEndpoint, err := hcsshim.GetHNSEndpointByName(endpointName)
	if err == nil && existingEndpoint.VirtualNetwork == dev.link.Id {
		// Check policies if there is PA type
		targetType := "PA"
		for _, policy := range existingEndpoint.Policies {
			policyType, _ := jsonparser.GetUnsafeString(policy, "Type")
			if policyType == targetType {
				actualPaIP, _ := jsonparser.GetUnsafeString(policy, targetType)
				if actualPaIP == n.ManagementAddress {
					// Found it and delete
					if _, err := existingEndpoint.Delete(); err != nil {
						return errors.Annotatef(err, "failed to delete remote HNSEndpoint %s", endpointName)
					}

					log.V(4).Infof("Deleted HNSEndpoint %s", endpointName)
					break
				}
			}
		}
	}

	return nil
}

func (dev *vxlanDevice) ConjureMac(targetIP ip.IP4) string {
	a, b, c, d := targetIP.Octets()
	return fmt.Sprintf("%v-%02x-%02x-%02x-%02x", dev.macPrefix, a, b, c, d)
}

func createEndpointName(targetIP ip.IP4) string {
	return "remote_" + targetIP.String()
}
