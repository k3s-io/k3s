package util

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/urfave/cli"
	apinet "k8s.io/apimachinery/pkg/util/net"
)

// JoinIPs stringifies and joins a list of IP addresses with commas.
func JoinIPs(elems []net.IP) string {
	var strs []string
	for _, elem := range elems {
		strs = append(strs, elem.String())
	}
	return strings.Join(strs, ",")
}

// JoinIPNets stringifies and joins a list of IP networks with commas.
func JoinIPNets(elems []*net.IPNet) string {
	var strs []string
	for _, elem := range elems {
		strs = append(strs, elem.String())
	}
	return strings.Join(strs, ",")
}

// GetFirst4Net returns the first IPv4 network from the list of IP networks.
// If no IPv4 addresses are found, an error is raised.
func GetFirst4Net(elems []*net.IPNet) (*net.IPNet, error) {
	for _, elem := range elems {
		if elem == nil || elem.IP.To4() == nil {
			continue
		}
		return elem, nil
	}
	return nil, errors.New("no IPv4 CIDRs found")
}

// GetFirst4 returns the first IPv4 address from the list of IP addresses.
// If no IPv4 addresses are found, an error is raised.
func GetFirst4(elems []net.IP) (net.IP, error) {
	for _, elem := range elems {
		if elem == nil || elem.To4() == nil {
			continue
		}
		return elem, nil
	}
	return nil, errors.New("no IPv4 address found")
}

// GetFirst4String returns the first IPv4 address from a list of IP address strings.
// If no IPv4 addresses are found, an error is raised.
func GetFirst4String(elems []string) (string, error) {
	ips := []net.IP{}
	for _, elem := range elems {
		for _, v := range strings.Split(elem, ",") {
			ips = append(ips, net.ParseIP(v))
		}
	}
	ip, err := GetFirst4(ips)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

// JoinIP4Nets stringifies and joins a list of IPv4 networks with commas.
func JoinIP4Nets(elems []*net.IPNet) string {
	var strs []string
	for _, elem := range elems {
		if elem != nil && elem.IP.To4() != nil {
			strs = append(strs, elem.String())
		}
	}
	return strings.Join(strs, ",")
}

// GetFirst6 returns the first IPv6 address from the list of IP addresses.
// If no IPv6 addresses are found, an error is raised.
func GetFirst6(elems []net.IP) (net.IP, error) {
	for _, elem := range elems {
		if elem == nil || elem.To16() == nil {
			continue
		}
		return elem, nil
	}
	return nil, errors.New("no IPv6 address found")
}

// GetFirst6Net returns the first IPv4 network from the list of IP networks.
// If no IPv6 addresses are found, an error is raised.
func GetFirst6Net(elems []*net.IPNet) (*net.IPNet, error) {
	for _, elem := range elems {
		if elem == nil || elem.IP.To16() == nil {
			continue
		}
		return elem, nil
	}
	return nil, errors.New("no IPv6 CIDRs found")
}

// GetFirst6String returns the first IPv6 address from a list of IP address strings.
// If no IPv6 addresses are found, an error is raised.
func GetFirst6String(elems []string) (string, error) {
	ips := []net.IP{}
	for _, elem := range elems {
		for _, v := range strings.Split(elem, ",") {
			ips = append(ips, net.ParseIP(v))
		}
	}
	ip, err := GetFirst6(ips)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

// JoinIP6Nets stringifies and joins a list of IPv6 networks with commas.
func JoinIP6Nets(elems []*net.IPNet) string {
	var strs []string
	for _, elem := range elems {
		if elem != nil && elem.IP.To4() == nil {
			strs = append(strs, elem.String())
		}
	}
	return strings.Join(strs, ",")
}

// GetHostnameAndIPs takes a node name and list of IPs, usually from CLI args.
// If set, these are used to return the node's name and addresses. If not set,
// the system hostname and primary interface address are returned instead.
func GetHostnameAndIPs(name string, nodeIPs cli.StringSlice) (string, []net.IP, error) {
	ips := []net.IP{}
	if len(nodeIPs) == 0 {
		hostIP, err := apinet.ChooseHostInterface()
		if err != nil {
			return "", nil, err
		}
		ips = append(ips, hostIP)
	} else {
		var err error
		ips, err = ParseStringSliceToIPs(nodeIPs)
		if err != nil {
			return "", nil, fmt.Errorf("invalid node-ip: %w", err)
		}
	}

	if name == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return "", nil, err
		}
		name = hostname
	}

	// Use lower case hostname to comply with kubernetes constraint:
	// https://github.com/kubernetes/kubernetes/issues/71140
	name = strings.ToLower(name)

	return name, ips, nil
}

// ParseStringSliceToIPs converts slice of strings that in turn can be lists of comma separated unparsed IP addresses
// into a single slice of net.IP, it returns error if at any point parsing failed
func ParseStringSliceToIPs(s cli.StringSlice) ([]net.IP, error) {
	var ips []net.IP
	for _, unparsedIP := range s {
		for _, v := range strings.Split(unparsedIP, ",") {
			ip := net.ParseIP(v)
			if ip == nil {
				return nil, fmt.Errorf("invalid ip format '%s'", v)
			}
			ips = append(ips, ip)
		}
	}

	return ips, nil
}

// GetFirstIP returns the first IPv4 address from the list of IP addresses.
// If no IPv4 addresses are found, returns the first IPv6 address
// if neither of IPv4 or IPv6 are found an error is raised.
// Additionally matching listen address and IP version is returned.
func GetFirstIP(nodeIPs []net.IP) (net.IP, string, bool, error) {
	nodeIP, err := GetFirst4(nodeIPs)
	ListenAddress := "0.0.0.0"
	IPv6only := false
	if err != nil {
		nodeIP, err = GetFirst6(nodeIPs)
		if err != nil {
			return nil, "", false, err
		}
		ListenAddress = "::"
		IPv6only = true
	}
	return nodeIP, ListenAddress, IPv6only, nil
}

// GetFirstNet returns the first IPv4 network from the list of IP networks.
// If no IPv4 addresses are found, returns the first IPv6 address
// if neither of IPv4 or IPv6 are found an error is raised.
func GetFirstNet(elems []*net.IPNet) (*net.IPNet, error) {
	serviceIPRange, err := GetFirst4Net(elems)
	if err != nil {
		serviceIPRange, err = GetFirst6Net(elems)
		if err != nil {
			return nil, err
		}
	}
	return serviceIPRange, nil
}

// GetFirstString returns the first IP4 address from a list of IP address strings.
// If no IPv4 addresses are found, returns the first IPv6 address
// if neither of IPv4 or IPv6 are found an error is raised.
func GetFirstString(elems []string) (string, bool, error) {
	ip, err := GetFirst4String(elems)
	IPv6only := false
	if err != nil {
		ip, err = GetFirst6String(elems)
		if err != nil {
			return "", false, err
		}
		IPv6only = true
	}
	return ip, IPv6only, nil
}

// IsIPv6OnlyCIDRs returns if
// - all are valid cidrs
// - at least one cidr from v6 family is found
// - v4 family cidr is not found
func IsIPv6OnlyCIDRs(cidrs []*net.IPNet) (bool, error) {
	v4Found := false
	v6Found := false
	for _, cidr := range cidrs {
		if cidr == nil {
			return false, fmt.Errorf("cidr %v is invalid", cidr)
		}

		if v4Found && v6Found {
			continue
		}

		if cidr.IP != nil && cidr.IP.To4() == nil {
			v6Found = true
			continue
		}
		v4Found = true
	}

	return !v4Found && v6Found, nil
}

// IPToIPNet converts an IP to an IPNet, using a fully filled mask appropriate for the address family.
func IPToIPNet(ip net.IP) (*net.IPNet, error) {
	address := ip.String()
	if strings.Contains(address, ":") {
		address += "/128"
	} else {
		address += "/32"
	}
	_, cidr, err := net.ParseCIDR(address)
	return cidr, err
}

// IPStringToIPNet converts an IP string to an IPNet, using a fully filled mask appropriate for the address family.
func IPStringToIPNet(address string) (*net.IPNet, error) {
	if strings.Contains(address, ":") {
		address += "/128"
	} else {
		address += "/32"
	}
	_, cidr, err := net.ParseCIDR(address)
	return cidr, err
}
