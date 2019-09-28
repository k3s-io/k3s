// Copyright 2017 CNI authors
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

package portmap

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
)

// fmtIpPort correctly formats ip:port literals for iptables and ip6tables -
// need to wrap v6 literals in a []
func fmtIpPort(ip net.IP, port int) string {
	if ip.To4() == nil {
		return fmt.Sprintf("[%s]:%d", ip.String(), port)
	}
	return fmt.Sprintf("%s:%d", ip.String(), port)
}

func localhostIP(isV6 bool) string {
	if isV6 {
		return "::1"
	}
	return "127.0.0.1"
}

// getRoutableHostIF will try and determine which interface routes the container's
// traffic. This is the one on which we disable martian filtering.
func getRoutableHostIF(containerIP net.IP) string {
	routes, err := netlink.RouteGet(containerIP)
	if err != nil {
		return ""
	}

	for _, route := range routes {
		link, err := netlink.LinkByIndex(route.LinkIndex)
		if err != nil {
			continue
		}

		return link.Attrs().Name
	}

	return ""
}

// groupByProto groups port numbers by protocol
func groupByProto(entries []PortMapEntry) map[string][]int {
	if len(entries) == 0 {
		return map[string][]int{}
	}
	out := map[string][]int{}
	for _, e := range entries {
		_, ok := out[e.Protocol]
		if ok {
			out[e.Protocol] = append(out[e.Protocol], e.HostPort)
		} else {
			out[e.Protocol] = []int{e.HostPort}
		}
	}

	return out
}

// splitPortList splits a list of integers in to one or more comma-separated
// string values, for use by multiport. Multiport only allows up to 15 ports
// per entry.
func splitPortList(l []int) []string {
	out := []string{}

	acc := []string{}
	for _, i := range l {
		acc = append(acc, strconv.Itoa(i))
		if len(acc) == 15 {
			out = append(out, strings.Join(acc, ","))
			acc = []string{}
		}
	}

	if len(acc) > 0 {
		out = append(out, strings.Join(acc, ","))
	}
	return out
}

// trimComment makes sure no comment is over the iptables limit of 255 chars
func trimComment(val string) string {
	if len(val) <= 255 {
		return val
	}

	return val[0:253] + "..."
}
