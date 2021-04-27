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
// +build windows

package vxlan

import (
	"encoding/json"
	"time"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	log "k8s.io/klog"
)

type vxlanDeviceAttrs struct {
	vni           uint32
	name          string
	gbp           bool
	addressPrefix ip.IP4Net
}

type vxlanDevice struct {
	link          *hcn.HostComputeNetwork
	macPrefix     string
	directRouting bool
}

func newVXLANDevice(devAttrs *vxlanDeviceAttrs) (*vxlanDevice, error) {
	subnet := createSubnet(devAttrs.addressPrefix.String(), (devAttrs.addressPrefix.IP + 1).String(), "0.0.0.0/0")
	network := &hcn.HostComputeNetwork{
		Type: "Overlay",
		Name: devAttrs.name,
		Ipams: []hcn.Ipam{
			{
				Type: "Static",
				Subnets: []hcn.Subnet{
					*subnet,
				},
			},
		},
		Flags: hcn.EnableNonPersistent,
		SchemaVersion: hcn.SchemaVersion{
			Major: 2,
			Minor: 0,
		},
	}

	vsid := &hcn.VsidPolicySetting{
		IsolationId: devAttrs.vni,
	}
	vsidJson, err := json.Marshal(vsid)
	if err != nil {
		return nil, err
	}

	sp := &hcn.SubnetPolicy{
		Type: hcn.VSID,
	}
	sp.Settings = vsidJson

	spJson, err := json.Marshal(sp)
	if err != nil {
		return nil, err
	}

	network.Ipams[0].Subnets[0].Policies = append(network.Ipams[0].Subnets[0].Policies, spJson)

	hnsNetwork, err := ensureNetwork(network, devAttrs.addressPrefix.String())
	if err != nil {
		return nil, err
	}

	return &vxlanDevice{
		link: hnsNetwork,
	}, nil
}

func ensureNetwork(expectedNetwork *hcn.HostComputeNetwork, expectedAddressPrefix string) (*hcn.HostComputeNetwork, error) {
	createNetwork := true
	networkName := expectedNetwork.Name

	// 1. Check if the HostComputeNetwork exists and has the expected settings
	existingNetwork, err := hcn.GetNetworkByName(networkName)
	if err == nil {
		if existingNetwork.Type == expectedNetwork.Type {
			if existingNetwork.Ipams[0].Subnets[0].IpAddressPrefix == expectedAddressPrefix {
				createNetwork = false
				log.Infof("Found existing HostComputeNetwork %s", networkName)
			}
		}
	}

	// 2. Create a new HNSNetwork
	if createNetwork {
		if existingNetwork != nil {
			if err := existingNetwork.Delete(); err != nil {
				return nil, errors.Wrapf(err, "failed to delete existing HostComputeNetwork %s", networkName)
			}
			log.Infof("Deleted stale HostComputeNetwork %s", networkName)
		}

		log.Infof("Attempting to create HostComputeNetwork %v", expectedNetwork)
		newNetwork, err := expectedNetwork.Create()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create HostComputeNetwork %s", networkName)
		}

		var waitErr, lastErr error
		// Wait for the network to populate Management IP
		log.Infof("Waiting to get ManagementIP from HostComputeNetwork %s", networkName)
		var newNetworkID = newNetwork.Id
		waitErr = wait.Poll(500*time.Millisecond, 5*time.Second, func() (done bool, err error) {
			newNetwork, lastErr = hcn.GetNetworkByID(newNetworkID)
			return newNetwork != nil && len(getManagementIP(newNetwork)) != 0, nil
		})
		if waitErr == wait.ErrWaitTimeout {
			// Do not swallow the root cause
			if lastErr != nil {
				waitErr = lastErr
			}
			return nil, errors.Wrapf(lastErr, "timeout, failed to get management IP from HostComputeNetwork %s", networkName)
		}

		managementIP := getManagementIP(newNetwork)
		// Wait for the interface with the management IP
		log.Infof("Waiting to get net interface for HostComputeNetwork %s (%s)", networkName, managementIP)
		managementIPv4, err := ip.ParseIP4(managementIP)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse management ip (%s)", managementIP)
		}

		waitErr = wait.Poll(500*time.Millisecond, 5*time.Second, func() (done bool, err error) {
			_, lastErr = ip.GetInterfaceByIP(managementIPv4.ToIP())
			return lastErr == nil, nil
		})
		if waitErr == wait.ErrWaitTimeout {
			return nil, errors.Wrapf(lastErr, "timeout, failed to get net interface for HostComputeNetwork %s (%s)", networkName, managementIP)
		}

		log.Infof("Created HostComputeNetwork %s", networkName)
		existingNetwork = newNetwork
	}

	addHostRoute := true
	for _, policy := range existingNetwork.Policies {
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
		err = existingNetwork.AddPolicy(networkRequest)
		if err != nil {
			log.Infof("Could not apply HostRoute policy for local host to local pod connectivity. This policy requires Windows 18321.1000.19h1_release.190117-1502 or newer")
		}
	}

	return existingNetwork, nil
}

func getManagementIP(network *hcn.HostComputeNetwork) string {
	for _, policy := range network.Policies {
		if policy.Type == hcn.ProviderAddress {
			policySettings := hcn.ProviderAddressEndpointPolicySetting{}
			err := json.Unmarshal(policy.Settings, &policySettings)
			if err != nil {
				return ""
			}
			return policySettings.ProviderAddress
		}
	}
	return ""
}

func createSubnet(AddressPrefix string, NextHop string, DestPrefix string) *hcn.Subnet {
	return &hcn.Subnet{
		IpAddressPrefix: AddressPrefix,
		Routes: []hcn.Route{
			{
				NextHop:           NextHop,
				DestinationPrefix: DestPrefix,
			},
		},
	}
}
