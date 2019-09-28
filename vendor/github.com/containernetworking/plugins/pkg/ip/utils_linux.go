// +build linux

// Copyright 2016 CNI authors
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
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/vishvananda/netlink"
)

func ValidateExpectedInterfaceIPs(ifName string, resultIPs []*current.IPConfig) error {

	// Ensure ips
	for _, ips := range resultIPs {
		ourAddr := netlink.Addr{IPNet: &ips.Address}
		match := false

		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("Cannot find container link %v", ifName)
		}

		addrList, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("Cannot obtain List of IP Addresses")
		}

		for _, addr := range addrList {
			if addr.Equal(ourAddr) {
				match = true
				break
			}
		}
		if match == false {
			return fmt.Errorf("Failed to match addr %v on interface %v", ourAddr, ifName)
		}

		// Convert the host/prefixlen to just prefix for route lookup.
		_, ourPrefix, err := net.ParseCIDR(ourAddr.String())

		findGwy := &netlink.Route{Dst: ourPrefix}
		routeFilter := netlink.RT_FILTER_DST
		var family int

		switch {
		case ips.Version == "4":
			family = netlink.FAMILY_V4
		case ips.Version == "6":
			family = netlink.FAMILY_V6
		default:
			return fmt.Errorf("Invalid IP Version %v for interface %v", ips.Version, ifName)
		}

		gwy, err := netlink.RouteListFiltered(family, findGwy, routeFilter)
		if err != nil {
			return fmt.Errorf("Error %v trying to find Gateway %v for interface %v", err, ips.Gateway, ifName)
		}
		if gwy == nil {
			return fmt.Errorf("Failed to find Gateway %v for interface %v", ips.Gateway, ifName)
		}
	}

	return nil
}

func ValidateExpectedRoute(resultRoutes []*types.Route) error {

	// Ensure that each static route in prevResults is found in the routing table
	for _, route := range resultRoutes {
		find := &netlink.Route{Dst: &route.Dst, Gw: route.GW}
		routeFilter := netlink.RT_FILTER_DST | netlink.RT_FILTER_GW
		var family int

		switch {
		case route.Dst.IP.To4() != nil:
			family = netlink.FAMILY_V4
			// Default route needs Dst set to nil
			if route.Dst.String() == "0.0.0.0/0" {
				find = &netlink.Route{Dst: nil, Gw: route.GW}
				routeFilter = netlink.RT_FILTER_DST
			}
		case len(route.Dst.IP) == net.IPv6len:
			family = netlink.FAMILY_V6
			// Default route needs Dst set to nil
			if route.Dst.String() == "::/0" {
				find = &netlink.Route{Dst: nil, Gw: route.GW}
				routeFilter = netlink.RT_FILTER_DST
			}
		default:
			return fmt.Errorf("Invalid static route found %v", route)
		}

		wasFound, err := netlink.RouteListFiltered(family, find, routeFilter)
		if err != nil {
			return fmt.Errorf("Expected Route %v not route table lookup error %v", route, err)
		}
		if wasFound == nil {
			return fmt.Errorf("Expected Route %v not found in routing table", route)
		}
	}

	return nil
}
