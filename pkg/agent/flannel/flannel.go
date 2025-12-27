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
	"errors"
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
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/merr"
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

	BackendNone            = "none"
	BackendVXLAN           = "vxlan"
	BackendHostGW          = "host-gw"
	BackendWireguardNative = "wireguard-native"
	BackendTailscale       = "tailscale"
)

var (
	BaseAnnotation         = "flannel.alpha.coreos.com"
	ExternalIPv4Annotation = BaseAnnotation + "/public-ip-overwrite"
	ExternalIPv6Annotation = BaseAnnotation + "/public-ipv6-overwrite"
)

func flannel(ctx context.Context, wg *sync.WaitGroup, flannelIface *net.Interface, flannelConf, kubeConfigFile string, flannelIPv6Masq bool, nm netMode) error {
	extIface, err := LookupExtInterface(flannelIface, nm)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to find the interface")
	}

	sm, err := kube.NewSubnetManager(ctx,
		"",
		kubeConfigFile,
		BaseAnnotation,
		flannelConf,
		false)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create the SubnetManager")
	}

	config, err := sm.GetNetworkConfig(ctx)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to get the network config")
	}

	// Create a backend manager then use it to create the backend and register the network with it.
	bm := backend.NewManager(ctx, sm, extIface)

	be, err := bm.GetBackend(config.BackendType)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create the flannel backend")
	}

	bn, err := be.RegisterNetwork(ctx, wg, config)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to register flannel network")
	}
	trafficMngr := &iptables.IPTablesManager{}
	err = trafficMngr.Init(ctx)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to initialize flannel ipTables manager")
	}

	if nm.IPv4Enabled() && config.Network.Empty() {
		return errors.New("ipv4 mode requested but no ipv4 network provided")
	}

	// setup masq rules
	prevNetwork := ReadCIDRFromSubnetFile(subnetFile, "FLANNEL_NETWORK")
	prevSubnet := ReadCIDRFromSubnetFile(subnetFile, "FLANNEL_SUBNET")

	prevIPv6Network := ReadIP6CIDRFromSubnetFile(subnetFile, "FLANNEL_IPV6_NETWORK")
	prevIPv6Subnet := ReadIP6CIDRFromSubnetFile(subnetFile, "FLANNEL_IPV6_SUBNET")
	if flannelIPv6Masq {
		err = trafficMngr.SetupAndEnsureMasqRules(ctx, config.Network, prevSubnet, prevNetwork, config.IPv6Network, prevIPv6Subnet, prevIPv6Network, bn.Lease(), 60, false)
	} else {
		// set empty flannel ipv6 Network to prevent masquerading
		err = trafficMngr.SetupAndEnsureMasqRules(ctx, config.Network, prevSubnet, prevNetwork, ip.IP6Net{}, prevIPv6Subnet, prevIPv6Network, bn.Lease(), 60, false)
	}
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to setup masq rules")
	}

	// setup forward rules
	trafficMngr.SetupAndEnsureForwardRules(ctx, config.Network, config.IPv6Network, 50)

	if err := WriteSubnetFile(subnetFile, config.Network, config.IPv6Network, true, bn, nm); err != nil {
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

func LookupExtInterface(iface *net.Interface, nm netMode) (*backend.ExternalInterface, error) {
	var ifaceAddr []net.IP
	var ifacev6Addr []net.IP
	var err error

	if iface == nil {
		logrus.Debug("No interface defined for flannel in the config. Fetching the default gateway interface")
		if nm.IPv4Enabled() {
			if iface, err = ip.GetDefaultGatewayInterface(); err != nil {
				return nil, pkgerrors.WithMessage(err, "failed to get default interface")
			}
		} else {
			if iface, err = ip.GetDefaultV6GatewayInterface(); err != nil {
				return nil, pkgerrors.WithMessage(err, "failed to get default interface")
			}
		}
	}
	logrus.Debugf("The interface %s will be used by flannel", iface.Name)

	if nm.IPv4Enabled() {
		ifaceAddr, err = ip.GetInterfaceIP4Addrs(iface)
		if err != nil {
			return nil, pkgerrors.WithMessagef(err, "failed to find IPv4 address for interface %s", iface.Name)
		}
		logrus.Infof("The interface %s with ipv4 address %s will be used by flannel", iface.Name, ifaceAddr[0])
	} else {
		ifaceAddr = append(ifaceAddr, nil)
	}
	if nm.IPv6Enabled() {
		ifacev6Addr, err = ip.GetInterfaceIP6Addrs(iface)
		if err != nil {
			return nil, pkgerrors.WithMessagef(err, "failed to find IPv6 address for interface %s", iface.Name)
		}
		logrus.Infof("The interface %s with ipv6 address %s will be used by flannel", iface.Name, ifacev6Addr[0])
	} else {
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

// WriteSubnetFile atomically writes the flannel subnet configuration file.
// Uses CreateTemp to avoid issues with pre-existing temp files (stale files
// from crashes, unexpected permissions/ownership) and to ensure clean atomic
// rename semantics with O_EXCL guarantees.
func WriteSubnetFile(path string, nw ip.IP4Net, nwv6 ip.IP6Net, ipMasq bool, bn backend.Network, nm netMode) error {
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Preserve original file permissions if the file already exists
	perm := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	f, err := os.CreateTemp(dir, "."+name+".")
	if err != nil {
		return err
	}
	tempFile := f.Name()
	cleanupNoClose := func(err error) error {
		return merr.NewErrors(err, os.Remove(tempFile))
	}
	cleanup := func(err error) error {
		return merr.NewErrors(err, f.Close(), os.Remove(tempFile))
	}
	if err := f.Chmod(perm); err != nil {
		return cleanup(err)
	}

	// Write out the first usable IP by incrementing
	// sn.IP by one
	sn := bn.Lease().Subnet
	sn.IP++
	if nm.IPv4Enabled() {
		if _, err := fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", nw); err != nil {
			return cleanup(err)
		}
		if _, err := fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn); err != nil {
			return cleanup(err)
		}
	}

	if nwv6.String() != emptyIPv6Network {
		snv6 := bn.Lease().IPv6Subnet
		snv6.IncrementIP()
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", nwv6); err != nil {
			return cleanup(err)
		}
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_SUBNET=%s\n", snv6); err != nil {
			return cleanup(err)
		}
	}

	if _, err := fmt.Fprintf(f, "FLANNEL_MTU=%d\n", bn.MTU()); err != nil {
		return cleanup(err)
	}
	if _, err := fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq); err != nil {
		return cleanup(err)
	}
	if err := f.Sync(); err != nil {
		return cleanup(err)
	}
	if err := f.Close(); err != nil {
		return cleanupNoClose(err)
	}

	// rename(2) the temporary file to the desired location so that it becomes
	// atomically visible with the contents (same directory keeps it on the same FS)
	if err := os.Rename(tempFile, path); err != nil {
		_ = os.Remove(tempFile)
		return err
	}
	return nil
}

// ReadCIDRFromSubnetFile reads the flannel subnet file and extracts the value of IPv4 network key
func ReadCIDRFromSubnetFile(path string, key string) ip.IP4Net {
	prevCIDRs := ReadCIDRsFromSubnetFile(path, key)
	if len(prevCIDRs) == 0 {
		logrus.Warningf("no subnet found for key: %s in file: %s", key, path)
		return ip.IP4Net{IP: 0, PrefixLen: 0}
	} else if len(prevCIDRs) > 1 {
		logrus.Errorf("error reading subnet: more than 1 entry found for key: %s in file %s: ", key, path)
		return ip.IP4Net{IP: 0, PrefixLen: 0}
	}
	return prevCIDRs[0]
}

func ReadCIDRsFromSubnetFile(path string, key string) []ip.IP4Net {
	prevCIDRs := make([]ip.IP4Net, 0)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			logrus.Errorf("Couldn't fetch previous %s from subnet file at %s: %v", key, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[key]; ok {
			cidrs := strings.Split(prevCIDRString, ",")
			prevCIDRs = make([]ip.IP4Net, 0)
			for i := range cidrs {
				_, cidr, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					logrus.Errorf("Couldn't parse previous %s from subnet file at %s: %v", key, path, err)
				}
				prevCIDRs = append(prevCIDRs, ip.FromIPNet(cidr))
			}
		}
	}
	return prevCIDRs
}

// ReadIP6CIDRFromSubnetFile reads the flannel subnet file and extracts the value of IPv6 network key
func ReadIP6CIDRFromSubnetFile(path string, key string) ip.IP6Net {
	prevCIDRs := ReadIP6CIDRsFromSubnetFile(path, key)
	if len(prevCIDRs) == 0 {
		logrus.Warningf("no subnet found for key: %s in file: %s", key, path)
		return ip.IP6Net{IP: (*ip.IP6)(big.NewInt(0)), PrefixLen: 0}
	} else if len(prevCIDRs) > 1 {
		logrus.Errorf("error reading subnet: more than 1 entry found for key: %s in file %s: ", key, path)
		return ip.IP6Net{IP: (*ip.IP6)(big.NewInt(0)), PrefixLen: 0}
	}
	return prevCIDRs[0]
}

func ReadIP6CIDRsFromSubnetFile(path string, key string) []ip.IP6Net {
	prevCIDRs := make([]ip.IP6Net, 0)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		prevSubnetVals, err := godotenv.Read(path)
		if err != nil {
			logrus.Errorf("Couldn't fetch previous %s from subnet file at %s: %v", key, path, err)
		} else if prevCIDRString, ok := prevSubnetVals[key]; ok {
			cidrs := strings.Split(prevCIDRString, ",")
			prevCIDRs = make([]ip.IP6Net, 0)
			for i := range cidrs {
				_, cidr, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					logrus.Errorf("Couldn't parse previous %s from subnet file at %s: %v", key, path, err)
				}
				prevCIDRs = append(prevCIDRs, ip.FromIP6Net(cidr))
			}
		}
	}
	return prevCIDRs
}
