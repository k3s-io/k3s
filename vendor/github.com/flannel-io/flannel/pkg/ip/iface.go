// +build !windows

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

package ip

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	log "k8s.io/klog"
)

func getIfaceAddrs(iface *net.Interface) ([]netlink.Addr, error) {
	link := &netlink.Device{
		netlink.LinkAttrs{
			Index: iface.Index,
		},
	}

	return netlink.AddrList(link, syscall.AF_INET)
}

func GetInterfaceIP4Addr(iface *net.Interface) (net.IP, error) {
	addrs, err := getIfaceAddrs(iface)
	if err != nil {
		return nil, err
	}

	// prefer non link-local addr
	var ll net.IP

	for _, addr := range addrs {
		if addr.IP.To4() == nil {
			continue
		}

		if addr.IP.IsGlobalUnicast() {
			return addr.IP, nil
		}

		if addr.IP.IsLinkLocalUnicast() {
			ll = addr.IP
		}
	}

	if ll != nil {
		// didn't find global but found link-local. it'll do.
		return ll, nil
	}

	return nil, errors.New("No IPv4 address found for given interface")
}

func GetInterfaceIP4AddrMatch(iface *net.Interface, matchAddr net.IP) error {
	addrs, err := getIfaceAddrs(iface)
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		// Attempt to parse the address in CIDR notation
		// and assert it is IPv4
		if addr.IP.To4() != nil {
			if addr.IP.To4().Equal(matchAddr) {
				return nil
			}
		}
	}

	return errors.New("No IPv4 address found for given interface")
}

func GetDefaultGatewayInterface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.New("Found default route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.New("Unable to find default route")
}

func GetInterfaceByIP(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		err := GetInterfaceIP4AddrMatch(&iface, ip)
		if err == nil {
			return &iface, nil
		}
	}

	return nil, errors.New("No interface with given IP found")
}

func DirectRouting(ip net.IP) (bool, error) {
	routes, err := netlink.RouteGet(ip)
	if err != nil {
		return false, fmt.Errorf("couldn't lookup route to %v: %v", ip, err)
	}
	if len(routes) == 1 && routes[0].Gw == nil {
		// There is only a single route and there's no gateway (i.e. it's directly connected)
		return true, nil
	}
	return false, nil
}

// EnsureV4AddressOnLink ensures that there is only one v4 Addr on `link` within the `ipn` address space and it equals `ipa`.
func EnsureV4AddressOnLink(ipa IP4Net, ipn IP4Net, link netlink.Link) error {
	addr := netlink.Addr{IPNet: ipa.ToIPNet()}
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	var hasAddr bool
	for _, existingAddr := range existingAddrs {
		if existingAddr.Equal(addr) {
			hasAddr = true
			continue
		}

		if ipn.Contains(FromIP(existingAddr.IP)) {
			if err := netlink.AddrDel(link, &existingAddr); err != nil {
				return fmt.Errorf("failed to remove IP address %s from %s: %s", existingAddr.String(), link.Attrs().Name, err)
			}
			log.Infof("removed IP address %s from %s", existingAddr.String(), link.Attrs().Name)
		}
	}

	// Actually add the desired address to the interface if needed.
	if !hasAddr {
		if err := netlink.AddrAdd(link, &addr); err != nil {
			return fmt.Errorf("failed to add IP address %s to %s: %s", addr.String(), link.Attrs().Name, err)
		}
	}

	return nil
}
