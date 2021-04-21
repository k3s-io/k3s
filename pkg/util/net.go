package util

import (
	"errors"
	"net"
	"strings"
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
