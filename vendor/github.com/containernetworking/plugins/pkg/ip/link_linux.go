// Copyright 2015 CNI authors
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
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/hwaddr"
	"github.com/vishvananda/netlink"
)

var (
	ErrLinkNotFound = errors.New("link not found")
)

func makeVethPair(name, peer string, mtu int) (netlink.Link, error) {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:  name,
			Flags: net.FlagUp,
			MTU:   mtu,
		},
		PeerName: peer,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, err
	}
	// Re-fetch the link to get its creation-time parameters, e.g. index and mac
	veth2, err := netlink.LinkByName(name)
	if err != nil {
		netlink.LinkDel(veth) // try and clean up the link if possible.
		return nil, err
	}

	return veth2, nil
}

func peerExists(name string) bool {
	if _, err := netlink.LinkByName(name); err != nil {
		return false
	}
	return true
}

func makeVeth(name string, mtu int) (peerName string, veth netlink.Link, err error) {
	for i := 0; i < 10; i++ {
		peerName, err = RandomVethName()
		if err != nil {
			return
		}

		veth, err = makeVethPair(name, peerName, mtu)
		switch {
		case err == nil:
			return

		case os.IsExist(err):
			if peerExists(peerName) {
				continue
			}
			err = fmt.Errorf("container veth name provided (%v) already exists", name)
			return

		default:
			err = fmt.Errorf("failed to make veth pair: %v", err)
			return
		}
	}

	// should really never be hit
	err = fmt.Errorf("failed to find a unique veth name")
	return
}

// RandomVethName returns string "veth" with random prefix (hashed from entropy)
func RandomVethName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Reader.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	// NetworkManager (recent versions) will ignore veth devices that start with "veth"
	return fmt.Sprintf("veth%x", entropy), nil
}

func RenameLink(curName, newName string) error {
	link, err := netlink.LinkByName(curName)
	if err == nil {
		err = netlink.LinkSetName(link, newName)
	}
	return err
}

func ifaceFromNetlinkLink(l netlink.Link) net.Interface {
	a := l.Attrs()
	return net.Interface{
		Index:        a.Index,
		MTU:          a.MTU,
		Name:         a.Name,
		HardwareAddr: a.HardwareAddr,
		Flags:        a.Flags,
	}
}

// SetupVeth sets up a pair of virtual ethernet devices.
// Call SetupVeth from inside the container netns.  It will create both veth
// devices and move the host-side veth into the provided hostNS namespace.
// On success, SetupVeth returns (hostVeth, containerVeth, nil)
func SetupVeth(contVethName string, mtu int, hostNS ns.NetNS) (net.Interface, net.Interface, error) {
	hostVethName, contVeth, err := makeVeth(contVethName, mtu)
	if err != nil {
		return net.Interface{}, net.Interface{}, err
	}

	if err = netlink.LinkSetUp(contVeth); err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to set %q up: %v", contVethName, err)
	}

	hostVeth, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to lookup %q: %v", hostVethName, err)
	}

	if err = netlink.LinkSetNsFd(hostVeth, int(hostNS.Fd())); err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to move veth to host netns: %v", err)
	}

	err = hostNS.Do(func(_ ns.NetNS) error {
		hostVeth, err = netlink.LinkByName(hostVethName)
		if err != nil {
			return fmt.Errorf("failed to lookup %q in %q: %v", hostVethName, hostNS.Path(), err)
		}

		if err = netlink.LinkSetUp(hostVeth); err != nil {
			return fmt.Errorf("failed to set %q up: %v", hostVethName, err)
		}
		return nil
	})
	if err != nil {
		return net.Interface{}, net.Interface{}, err
	}
	return ifaceFromNetlinkLink(hostVeth), ifaceFromNetlinkLink(contVeth), nil
}

// DelLinkByName removes an interface link.
func DelLinkByName(ifName string) error {
	iface, err := netlink.LinkByName(ifName)
	if err != nil {
		if err.Error() == "Link not found" {
			return ErrLinkNotFound
		}
		return fmt.Errorf("failed to lookup %q: %v", ifName, err)
	}

	if err = netlink.LinkDel(iface); err != nil {
		return fmt.Errorf("failed to delete %q: %v", ifName, err)
	}

	return nil
}

// DelLinkByNameAddr remove an interface and returns its addresses
func DelLinkByNameAddr(ifName string) ([]*net.IPNet, error) {
	iface, err := netlink.LinkByName(ifName)
	if err != nil {
		if err != nil && err.Error() == "Link not found" {
			return nil, ErrLinkNotFound
		}
		return nil, fmt.Errorf("failed to lookup %q: %v", ifName, err)
	}

	addrs, err := netlink.AddrList(iface, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP addresses for %q: %v", ifName, err)
	}

	if err = netlink.LinkDel(iface); err != nil {
		return nil, fmt.Errorf("failed to delete %q: %v", ifName, err)
	}

	out := []*net.IPNet{}
	for _, addr := range addrs {
		if addr.IP.IsGlobalUnicast() {
			out = append(out, addr.IPNet)
		}
	}

	return out, nil
}

func SetHWAddrByIP(ifName string, ip4 net.IP, ip6 net.IP) error {
	iface, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", ifName, err)
	}

	switch {
	case ip4 == nil && ip6 == nil:
		return fmt.Errorf("neither ip4 or ip6 specified")

	case ip4 != nil:
		{
			hwAddr, err := hwaddr.GenerateHardwareAddr4(ip4, hwaddr.PrivateMACPrefix)
			if err != nil {
				return fmt.Errorf("failed to generate hardware addr: %v", err)
			}
			if err = netlink.LinkSetHardwareAddr(iface, hwAddr); err != nil {
				return fmt.Errorf("failed to add hardware addr to %q: %v", ifName, err)
			}
		}
	case ip6 != nil:
		// TODO: IPv6
	}

	return nil
}
