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

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/iptables"
	"github.com/flannel-io/flannel/pkg/subnet/kube"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	// Backends need to be imported for their init() to get executed and them to register
	_ "github.com/flannel-io/flannel/pkg/backend/extension"
	_ "github.com/flannel-io/flannel/pkg/backend/hostgw"
	_ "github.com/flannel-io/flannel/pkg/backend/ipsec"
	_ "github.com/flannel-io/flannel/pkg/backend/vxlan"
	_ "github.com/flannel-io/flannel/pkg/backend/wireguard"
)

const (
	subnetFile = "/run/flannel/subnet.env"
)

var (
	FlannelBaseAnnotation         = "flannel.alpha.coreos.com"
	FlannelExternalIPv4Annotation = FlannelBaseAnnotation + "/public-ip-overwrite"
	FlannelExternalIPv6Annotation = FlannelBaseAnnotation + "/public-ipv6-overwrite"
)

func flannel(ctx context.Context, flannelIface *net.Interface, flannelConf, kubeConfigFile string, flannelIPv6Masq bool, netMode int) error {
	extIface, err := LookupExtInterface(flannelIface, netMode)
	if err != nil {
		return errors.Wrap(err, "failed to find the interface")
	}

	sm, err := kube.NewSubnetManager(ctx,
		"",
		kubeConfigFile,
		FlannelBaseAnnotation,
		flannelConf,
		false)
	if err != nil {
		return errors.Wrap(err, "failed to create the SubnetManager")
	}

	config, err := sm.GetNetworkConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the network config")
	}

	// Create a backend manager then use it to create the backend and register the network with it.
	bm := backend.NewManager(ctx, sm, extIface)

	be, err := bm.GetBackend(config.BackendType)
	if err != nil {
		return errors.Wrap(err, "failed to create the flannel backend")
	}

	bn, err := be.RegisterNetwork(ctx, &sync.WaitGroup{}, config)
	if err != nil {
		return errors.Wrap(err, "failed to register flannel network")
	}

	if netMode == (ipv4+ipv6) || netMode == ipv4 {
		net, err := config.GetFlannelNetwork(&bn.Lease().Subnet)
		if err != nil {
			return errors.Wrap(err, "failed to get flannel network details")
		}
		iptables.CreateIP4Chain("nat", "FLANNEL-POSTRTG")
		iptables.CreateIP4Chain("filter", "FLANNEL-FWD")
		getMasqRules := func() []iptables.IPTablesRule {
			if config.HasNetworks() {
				return iptables.MasqRules(config.Networks, bn.Lease())
			}
			return iptables.MasqRules([]ip.IP4Net{config.Network}, bn.Lease())
		}
		getFwdRules := func() []iptables.IPTablesRule {
			return iptables.ForwardRules(net.String())
		}
		go iptables.SetupAndEnsureIP4Tables(getMasqRules, 60)
		go iptables.SetupAndEnsureIP4Tables(getFwdRules, 50)
	}

	if config.IPv6Network.String() != emptyIPv6Network {
		ip6net, err := config.GetFlannelIPv6Network(&bn.Lease().IPv6Subnet)
		if err != nil {
			return errors.Wrap(err, "failed to get ipv6 flannel network details")
		}
		if flannelIPv6Masq {
			logrus.Debugf("Creating IPv6 masquerading iptables rules for %s network", config.IPv6Network.String())
			iptables.CreateIP6Chain("nat", "FLANNEL-POSTRTG")
			getRules := func() []iptables.IPTablesRule {
				if config.HasIPv6Networks() {
					return iptables.MasqIP6Rules(config.IPv6Networks, bn.Lease())
				}
				return iptables.MasqIP6Rules([]ip.IP6Net{config.IPv6Network}, bn.Lease())
			}
			go iptables.SetupAndEnsureIP6Tables(getRules, 60)
		}
		iptables.CreateIP6Chain("filter", "FLANNEL-FWD")
		getRules := func() []iptables.IPTablesRule {
			return iptables.ForwardRules(ip6net.String())
		}
		go iptables.SetupAndEnsureIP6Tables(getRules, 50)
	}

	if err := WriteSubnetFile(subnetFile, config.Network, config.IPv6Network, true, bn, netMode); err != nil {
		// Continue, even though it failed.
		logrus.Warningf("Failed to write flannel subnet file: %s", err)
	} else {
		logrus.Infof("Wrote flannel subnet file to %s", subnetFile)
	}

	// Start "Running" the backend network. This will block until the context is done so run in another goroutine.
	logrus.Info("Running flannel backend.")
	bn.Run(ctx)
	return nil
}

func LookupExtInterface(iface *net.Interface, netMode int) (*backend.ExternalInterface, error) {
	var ifaceAddr []net.IP
	var ifacev6Addr []net.IP
	var err error

	if iface == nil {
		logrus.Debug("No interface defined for flannel in the config. Fetching the default gateway interface")
		if netMode == ipv4 || netMode == (ipv4+ipv6) {
			if iface, err = ip.GetDefaultGatewayInterface(); err != nil {
				return nil, errors.Wrap(err, "failed to get default interface")
			}
		} else {
			if iface, err = ip.GetDefaultV6GatewayInterface(); err != nil {
				return nil, errors.Wrap(err, "failed to get default interface")
			}
		}
	}
	logrus.Debugf("The interface %s will be used by flannel", iface.Name)

	switch netMode {
	case ipv4:
		ifaceAddr, err = ip.GetInterfaceIP4Addrs(iface)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find IPv4 address for interface")
		}
		logrus.Infof("The interface %s with ipv4 address %s will be used by flannel", iface.Name, ifaceAddr[0])
		ifacev6Addr = append(ifacev6Addr, nil)
	case ipv6:
		ifacev6Addr, err = ip.GetInterfaceIP6Addrs(iface)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find IPv6 address for interface")
		}
		logrus.Infof("The interface %s with ipv6 address %s will be used by flannel", iface.Name, ifacev6Addr[0])
		ifaceAddr = append(ifaceAddr, nil)
	case (ipv4 + ipv6):
		ifaceAddr, err = ip.GetInterfaceIP4Addrs(iface)
		if err != nil {
			return nil, fmt.Errorf("failed to find IPv4 address for interface %s", iface.Name)
		}
		ifacev6Addr, err = ip.GetInterfaceIP6Addrs(iface)
		if err != nil {
			return nil, fmt.Errorf("failed to find IPv6 address for interface %s", iface.Name)
		}
		logrus.Infof("Using dual-stack mode. The interface %s with ipv4 address %s and ipv6 address %s will be used by flannel", iface.Name, ifaceAddr[0], ifacev6Addr[0])
	default:
		ifaceAddr = append(ifaceAddr, nil)
		ifacev6Addr = append(ifacev6Addr, nil)
	}

	if iface.MTU == 0 {
		return nil, fmt.Errorf("failed to determine MTU for %s interface", iface.Name)
	}

	return &backend.ExternalInterface{
		Iface:       iface,
		IfaceAddr:   ifaceAddr[0],
		IfaceV6Addr: ifacev6Addr[0],
		ExtAddr:     ifaceAddr[0],
		ExtV6Addr:   ifacev6Addr[0],
	}, nil
}

func WriteSubnetFile(path string, nw ip.IP4Net, nwv6 ip.IP6Net, ipMasq bool, bn backend.Network, netMode int) error {
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
	if netMode == ipv4 || netMode == (ipv4+ipv6) {
		fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", nw)
		fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn)
	}

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
