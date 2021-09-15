//
// Copyright 2015 flannel authors
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

package flannel

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/network"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet/kube"
	"golang.org/x/net/context"
	log "k8s.io/klog"

	// Backends need to be imported for their init() to get executed and them to register
	_ "github.com/flannel-io/flannel/backend/extension"
	_ "github.com/flannel-io/flannel/backend/hostgw"
	_ "github.com/flannel-io/flannel/backend/ipsec"
	_ "github.com/flannel-io/flannel/backend/vxlan"
)

const (
	subnetFile = "/run/flannel/subnet.env"
)

func flannel(ctx context.Context, flannelIface *net.Interface, flannelConf, kubeConfigFile string, netMode int) error {
	extIface, err := LookupExtInterface(flannelIface, netMode)
	if err != nil {
		return err
	}

	sm, err := kube.NewSubnetManager(ctx, "", kubeConfigFile, "flannel.alpha.coreos.com", flannelConf, false)
	if err != nil {
		return err
	}

	config, err := sm.GetNetworkConfig(ctx)
	if err != nil {
		return err
	}

	// Create a backend manager then use it to create the backend and register the network with it.
	bm := backend.NewManager(ctx, sm, extIface)

	be, err := bm.GetBackend(config.BackendType)
	if err != nil {
		return err
	}

	bn, err := be.RegisterNetwork(ctx, &sync.WaitGroup{}, config)
	if err != nil {
		return err
	}

	go network.SetupAndEnsureIPTables(network.MasqRules(config.Network, bn.Lease()), 60)
	go network.SetupAndEnsureIPTables(network.ForwardRules(config.Network.String()), 50)

	if err := WriteSubnetFile(subnetFile, config.Network, config.IPv6Network, true, bn); err != nil {
		// Continue, even though it failed.
		log.Warningf("Failed to write subnet file: %s", err)
	} else {
		log.Infof("Wrote subnet file to %s", subnetFile)
	}

	// Start "Running" the backend network. This will block until the context is done so run in another goroutine.
	log.Info("Running backend.")
	bn.Run(ctx)
	return nil
}

func LookupExtInterface(iface *net.Interface, netMode int) (*backend.ExternalInterface, error) {
	var ifaceAddr net.IP
	var ifacev6Addr net.IP
	var err error

	if iface == nil {
		log.Info("Determining IP address of default interface")
		if iface, err = ip.GetDefaultGatewayInterface(); err != nil {
			return nil, fmt.Errorf("failed to get default interface: %s", err)
		}
	} else {
		log.Info("Determining IP address of specified interface: ", iface.Name)
	}

	ifaceAddr, err = ip.GetInterfaceIP4Addr(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to find IPv4 address for interface %s", iface.Name)
	}

	if netMode == (ipv4 + ipv6) {
		ifacev6Addr, err = ip.GetInterfaceIP6Addr(iface)
		if err != nil {
			return nil, fmt.Errorf("failed to find IPv6 address for interface %s", iface.Name)
		}

		log.Infof("Using ipv6 address %s", ifacev6Addr)
	}
	if iface.MTU == 0 {
		return nil, fmt.Errorf("failed to determine MTU for %s interface", ifaceAddr)
	}

	return &backend.ExternalInterface{
		Iface:       iface,
		IfaceAddr:   ifaceAddr,
		IfaceV6Addr: ifacev6Addr,
		ExtAddr:     ifaceAddr,
		ExtV6Addr:   ifacev6Addr,
	}, nil
}

func WriteSubnetFile(path string, nw ip.IP4Net, nwv6 ip.IP6Net, ipMasq bool, bn backend.Network) error {
	dir, name := filepath.Split(path)
	os.MkdirAll(dir, 0755)

	tempFile := filepath.Join(dir, "."+name)
	f, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	// Write out the first usable IP by incrementing
	// sn.IP by one
	sn := bn.Lease().Subnet
	sn.IP++

	fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", nw)
	fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn)

	if nwv6.String() != emptyIPv6Network {
		snv6 := bn.Lease().IPv6Subnet
		snv6.IncrementIP()
		fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", nwv6)
		fmt.Fprintf(f, "FLANNEL_IPV6_SUBNET=%s\n", snv6)
	}

	fmt.Fprintf(f, "FLANNEL_MTU=%d\n", bn.MTU())
	_, err = fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq)
	f.Close()
	if err != nil {
		return err
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents
	return os.Rename(tempFile, path)
	//TODO - is this safe? What if it's not on the same FS?
}
