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
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/pkg/subnet/kube"
	"github.com/flannel-io/flannel/pkg/trafficmngr/iptables"
	"github.com/joho/godotenv"
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
	trafficMngr := &iptables.IPTablesManager{}
	err = trafficMngr.Init(ctx, &sync.WaitGroup{})
	if err != nil {
		return errors.Wrap(err, "failed to initialize flannel ipTables manager")
	}

	if netMode == (ipv4+ipv6) || netMode == ipv4 {
		if config.Network.Empty() {
			return errors.New("ipv4 mode requested but no ipv4 network provided")
		}
	}

	//setup masq rules
	prevNetwork := ReadCIDRFromSubnetFile(subnetFile, "FLANNEL_NETWORK")
	prevSubnet := ReadCIDRFromSubnetFile(subnetFile, "FLANNEL_SUBNET")

	prevIPv6Network := ReadIP6CIDRFromSubnetFile(subnetFile, "FLANNEL_IPV6_NETWORK")
	prevIPv6Subnet := ReadIP6CIDRFromSubnetFile(subnetFile, "FLANNEL_IPV6_SUBNET")
	if flannelIPv6Masq {
		err = trafficMngr.SetupAndEnsureMasqRules(ctx, config.Network, prevSubnet, prevNetwork, config.IPv6Network, prevIPv6Subnet, prevIPv6Network, bn.Lease(), 60)
	} else {
		//set empty flannel ipv6 Network to prevent masquerading
		err = trafficMngr.SetupAndEnsureMasqRules(ctx, config.Network, prevSubnet, prevNetwork, ip.IP6Net{}, prevIPv6Subnet, prevIPv6Network, bn.Lease(), 60)
	}
	if err != nil {
		return errors.Wrap(err, "failed to setup masq rules")
	}

	//setup forward rules
	trafficMngr.SetupAndEnsureForwardRules(ctx, config.Network, config.IPv6Network, 50)

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

// ReadCIDRFromSubnetFile reads the flannel subnet file and extracts the value of IPv4 network CIDRKey
func ReadCIDRFromSubnetFile(path string, CIDRKey string) ip.IP4Net {
	prevCIDRs := ReadCIDRsFromSubnetFile(path, CIDRKey)
	if len(prevCIDRs) == 0 {
		logrus.Warningf("no subnet found for key: %s in file: %s", CIDRKey, path)
		return ip.IP4Net{IP: 0, PrefixLen: 0}
	} else if len(prevCIDRs) > 1 {
		logrus.Errorf("error reading subnet: more than 1 entry found for key: %s in file %s: ", CIDRKey, path)
		return ip.IP4Net{IP: 0, PrefixLen: 0}
	} else {
		return prevCIDRs[0]
	}
}

func ReadCIDRsFromSubnetFile(path string, CIDRKey string) []ip.IP4Net {
	prevCIDRs := make([]ip.IP4Net, 0)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			logrus.Errorf("Couldn't fetch previous %s from subnet file at %s: %v", CIDRKey, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[CIDRKey]; ok {
			cidrs := strings.Split(prevCIDRString, ",")
			prevCIDRs = make([]ip.IP4Net, 0)
			for i := range cidrs {
				_, cidr, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					logrus.Errorf("Couldn't parse previous %s from subnet file at %s: %v", CIDRKey, path, err)
				}
				prevCIDRs = append(prevCIDRs, ip.FromIPNet(cidr))
			}

		}
	}
	return prevCIDRs
}


// ReadIP6CIDRFromSubnetFile reads the flannel subnet file and extracts the value of IPv6 network CIDRKey
func ReadIP6CIDRFromSubnetFile(path string, CIDRKey string) ip.IP6Net {
	prevCIDRs := ReadIP6CIDRsFromSubnetFile(path, CIDRKey)
	if len(prevCIDRs) == 0 {
		logrus.Warningf("no subnet found for key: %s in file: %s", CIDRKey, path)
		return ip.IP6Net{IP: (*ip.IP6)(big.NewInt(0)), PrefixLen: 0}
	} else if len(prevCIDRs) > 1 {
		logrus.Errorf("error reading subnet: more than 1 entry found for key: %s in file %s: ", CIDRKey, path)
		return ip.IP6Net{IP: (*ip.IP6)(big.NewInt(0)), PrefixLen: 0}
	} else {
		return prevCIDRs[0]
	}
}

func ReadIP6CIDRsFromSubnetFile(path string, CIDRKey string) []ip.IP6Net {
	prevCIDRs := make([]ip.IP6Net, 0)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			logrus.Errorf("Couldn't fetch previous %s from subnet file at %s: %v", CIDRKey, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[CIDRKey]; ok {
			cidrs := strings.Split(prevCIDRString, ",")
			prevCIDRs = make([]ip.IP6Net, 0)
			for i := range cidrs {
				_, cidr, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					logrus.Errorf("Couldn't parse previous %s from subnet file at %s: %v", CIDRKey, path, err)
				}
				prevCIDRs = append(prevCIDRs, ip.FromIP6Net(cidr))
			}

		}
	}
	return prevCIDRs
}
