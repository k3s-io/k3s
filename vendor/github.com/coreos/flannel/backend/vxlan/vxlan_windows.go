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

// Some design notes:
// VXLAN encapsulates L2 packets (though flannel is L3 only so don't expect to be able to send L2 packets across hosts)
// Windows overlay decap works at L2 and so it needs the correct destination MAC for the remote host to work.
// Windows does not expose an L3Miss interface so for now all possible remote IP/MAC pairs have to be configured upfront.
//
// In this scheme the scaling of table entries (per host) is:
//  - 1 network entry for the overlay network
//  - 1 endpoint per local container
//  - N remote endpoints remote node (total endpoints =
import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	log "k8s.io/klog"

	"golang.org/x/net/context"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/coreos/flannel/backend"
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
	"net"
)

func init() {
	backend.Register("vxlan", New)
}

const (
	defaultVNI = 4096
	vxlanPort  = 4789
)

type VXLANBackend struct {
	subnetMgr subnet.Manager
	extIface  *backend.ExternalInterface
}

func New(sm subnet.Manager, extIface *backend.ExternalInterface) (backend.Backend, error) {
	backend := &VXLANBackend{
		subnetMgr: sm,
		extIface:  extIface,
	}

	return backend, nil
}

func newSubnetAttrs(publicIP net.IP, vnid uint16, mac net.HardwareAddr) (*subnet.LeaseAttrs, error) {
	var hardwareAddress hardwareAddr
	if mac != nil {
		hardwareAddress = hardwareAddr(mac)
	}
	leaseAttrs := &vxlanLeaseAttrs{
		VNI:     vnid,
		VtepMAC: hardwareAddress,
	}
	data, err := json.Marshal(&leaseAttrs)
	if err != nil {
		return nil, err
	}

	return &subnet.LeaseAttrs{
		PublicIP:    ip.FromIP(publicIP),
		BackendType: "vxlan",
		BackendData: json.RawMessage(data),
	}, nil
}

func (be *VXLANBackend) RegisterNetwork(ctx context.Context, wg sync.WaitGroup, config *subnet.Config) (backend.Network, error) {
	// 1. Parse configuration
	cfg := struct {
		Name          string
		MacPrefix     string
		VNI           int
		Port          int
		GBP           bool
		DirectRouting bool
	}{
		VNI:       defaultVNI,
		Port:      vxlanPort,
		MacPrefix: "0E-2A",
	}

	if len(config.Backend) > 0 {
		if err := json.Unmarshal(config.Backend, &cfg); err != nil {
			return nil, fmt.Errorf("error decoding VXLAN backend config: %v", err)
		}
	}

	// 2. Verify configuration
	if cfg.VNI < defaultVNI {
		return nil, fmt.Errorf("invalid VXLAN backend config. VNI [%v] must be greater than or equal to %v on Windows", cfg.VNI, defaultVNI)
	}
	if cfg.Port != vxlanPort {
		return nil, fmt.Errorf("invalid VXLAN backend config. Port [%v] is not supported on Windows. Omit the setting to default to port %v", cfg.Port, vxlanPort)
	}
	if cfg.DirectRouting {
		return nil, errors.New("invalid VXLAN backend config. DirectRouting is not supported on Windows")
	}
	if cfg.GBP {
		return nil, errors.New("invalid VXLAN backend config. GBP is not supported on Windows")
	}
	if len(cfg.MacPrefix) == 0 || len(cfg.MacPrefix) != 5 || cfg.MacPrefix[2] != '-' {
		return nil, fmt.Errorf("invalid VXLAN backend config.MacPrefix [%v] is invalid, prefix must be of the format xx-xx e.g. 0E-2A", cfg.MacPrefix)
	}
	if len(cfg.Name) == 0 {
		cfg.Name = fmt.Sprintf("flannel.%v", cfg.VNI)
	}
	log.Infof("VXLAN config: Name=%s MacPrefix=%s VNI=%d Port=%d GBP=%v DirectRouting=%v", cfg.Name, cfg.MacPrefix, cfg.VNI, cfg.Port, cfg.GBP, cfg.DirectRouting)

	err := hcn.RemoteSubnetSupported()
	if err != nil {
		return nil, err
	}

	subnetAttrs, err := newSubnetAttrs(be.extIface.ExtAddr, uint16(cfg.VNI), nil)
	if err != nil {
		return nil, err
	}

	lease, err := be.subnetMgr.AcquireLease(ctx, subnetAttrs)
	switch err {
	case nil:
	case context.Canceled, context.DeadlineExceeded:
		return nil, err
	default:
		return nil, fmt.Errorf("failed to acquire lease: %v", err)
	}

	devAttrs := vxlanDeviceAttrs{
		vni:           uint32(cfg.VNI),
		name:          cfg.Name,
		addressPrefix: lease.Subnet,
	}

	dev, err := newVXLANDevice(&devAttrs)
	if err != nil {
		return nil, err
	}
	dev.directRouting = cfg.DirectRouting
	dev.macPrefix = cfg.MacPrefix

	network, err := newNetwork(be.subnetMgr, be.extIface, dev, ip.IP4Net{}, lease)
	if err != nil {
		return nil, err
	}

	hcnNetwork, err := hcn.GetNetworkByName(cfg.Name)
	if err != nil {
		return nil, err
	}

	var newDrMac string
	for _, policy := range hcnNetwork.Policies {
		if policy.Type == hcn.DrMacAddress {
			policySettings := hcn.DrMacAddressNetworkPolicySetting{}
			err = json.Unmarshal(policy.Settings, &policySettings)
			if err != nil {
				return nil, fmt.Errorf("Failed to unmarshal settings")
			}
			newDrMac = policySettings.Address
		}
	}

	mac, err := net.ParseMAC(string(newDrMac))
	if err != nil {
		return nil, fmt.Errorf("Cannot parse DR MAC %v: %+v", newDrMac, err)
	}

	subnetAttrs, err = newSubnetAttrs(be.extIface.ExtAddr, uint16(cfg.VNI), mac)
	if err != nil {
		return nil, err
	}

	lease, err = be.subnetMgr.AcquireLease(ctx, subnetAttrs)

	return network, nil
}

// So we can make it JSON (un)marshalable
type hardwareAddr net.HardwareAddr

func (hw hardwareAddr) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", net.HardwareAddr(hw))), nil
}

func (hw *hardwareAddr) UnmarshalJSON(bytes []byte) error {
	if len(bytes) < 2 || bytes[0] != '"' || bytes[len(bytes)-1] != '"' {
		return fmt.Errorf("error parsing hardware addr")
	}

	bytes = bytes[1 : len(bytes)-1]

	mac, err := net.ParseMAC(string(bytes))
	if err != nil {
		return err
	}

	*hw = hardwareAddr(mac)
	return nil
}
