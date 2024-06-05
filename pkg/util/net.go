package util

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	apinet "k8s.io/apimachinery/pkg/util/net"
	netutils "k8s.io/utils/net"
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

// getFirst4Net returns the first IPv4 network from the list of IP networks.
// If no IPv4 addresses are found, an error is raised.
func getFirst4Net(elems []*net.IPNet) (*net.IPNet, error) {
	for _, elem := range elems {
		if elem == nil || elem.IP.To4() == nil {
			continue
		}
		return elem, nil
	}
	return nil, errors.New("no IPv4 CIDRs found")
}

// getFirst4 returns the first IPv4 address from the list of IP addresses.
// If no IPv4 addresses are found, an error is raised.
func getFirst4(elems []net.IP) (net.IP, error) {
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
	ip, err := getFirst4(ips)
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

// getFirst6 returns the first IPv6 address from the list of IP addresses.
// If no IPv6 addresses are found, an error is raised.
func getFirst6(elems []net.IP) (net.IP, error) {
	for _, elem := range elems {
		if elem != nil && netutils.IsIPv6(elem) {
			return elem, nil
		}
	}
	return nil, errors.New("no IPv6 address found")
}

// getFirst6Net returns the first IPv4 network from the list of IP networks.
// If no IPv6 addresses are found, an error is raised.
func getFirst6Net(elems []*net.IPNet) (*net.IPNet, error) {
	for _, elem := range elems {
		if elem != nil && netutils.IsIPv6(elem.IP) {
			return elem, nil
		}
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
	ip, err := getFirst6(ips)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

// JoinIP6Nets stringifies and joins a list of IPv6 networks with commas.
func JoinIP6Nets(elems []*net.IPNet) string {
	var strs []string
	for _, elem := range elems {
		if elem != nil && netutils.IsIPv6(elem.IP) {
			strs = append(strs, elem.String())
		}
	}
	return strings.Join(strs, ",")
}

// GetHostnameAndIPs takes a node name and list of IPs, usually from CLI args.
// If set, these are used to return the node's name and addresses. If not set,
// the system hostname and primary interface addresses are returned instead.
func GetHostnameAndIPs(name string, nodeIPs cli.StringSlice) (string, []net.IP, error) {
	ips := []net.IP{}
	if len(nodeIPs) == 0 {
		hostIP, err := apinet.ChooseHostInterface()
		if err != nil {
			return "", nil, err
		}
		ips = append(ips, hostIP)
		// If IPv6 it's an IPv6 only node
		if hostIP.To4() != nil {
			hostIPv6, err := apinet.ResolveBindAddress(net.IPv6loopback)
			if err == nil && !hostIPv6.Equal(hostIP) {
				ips = append(ips, hostIPv6)
			}
		}
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

// GetFirstValidIPString returns the first valid address from a list of IP address strings,
// without preference for IP family. If no address are found, an empty string is returned.
func GetFirstValidIPString(s cli.StringSlice) string {
	for _, unparsedIP := range s {
		for _, v := range strings.Split(unparsedIP, ",") {
			if ip := net.ParseIP(v); ip != nil {
				return v
			}
		}
	}
	return ""
}

// GetFirstIP checks what is the IPFamily of the first item. Based on that, returns a set of values
func GetDefaultAddresses(nodeIP net.IP) (string, string, string, error) {

	if netutils.IsIPv4(nodeIP) {
		ListenAddress := "0.0.0.0"
		clusterCIDR := "10.42.0.0/16"
		serviceCIDR := "10.43.0.0/16"

		return ListenAddress, clusterCIDR, serviceCIDR, nil
	}

	if netutils.IsIPv6(nodeIP) {
		ListenAddress := "::"
		clusterCIDR := "fd00:42::/56"
		serviceCIDR := "fd00:43::/112"

		return ListenAddress, clusterCIDR, serviceCIDR, nil
	}

	return "", "", "", fmt.Errorf("ip: %v is not ipv4 or ipv6", nodeIP)
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

// GetIPFromInterface is the public function that returns the IP of an interface
func GetIPFromInterface(ifaceName string) (string, error) {
	ip, err := getIPFromInterface(ifaceName)
	if err != nil {
		return "", fmt.Errorf("interface %s does not have a correct global unicast ip: %w", ifaceName, err)
	}
	logrus.Infof("Found ip %s from iface %s", ip, ifaceName)
	return ip, nil
}

// getIPFromInterface is the private function that returns de IP of an interface
func getIPFromInterface(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	if iface.Flags&net.FlagUp == 0 {
		return "", fmt.Errorf("the interface %s is not up", ifaceName)
	}

	globalUnicasts := []string{}
	globalUnicastsIPv6 := []string{}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return "", fmt.Errorf("unable to parse CIDR for interface %s: %w", iface.Name, err)
		}
		// if not IPv4 adding it on IPv6 list
		if ip.To4() == nil {
			if ip.IsGlobalUnicast() {
				globalUnicastsIPv6 = append(globalUnicastsIPv6, ip.String())
			}
			continue
		}
		if ip.IsGlobalUnicast() {
			globalUnicasts = append(globalUnicasts, ip.String())
		}
	}

	if len(globalUnicasts) > 1 {
		return "", fmt.Errorf("multiple global unicast addresses defined for %s, please set ip from one of %v", ifaceName, globalUnicasts)
	}
	if len(globalUnicasts) == 1 && len(globalUnicastsIPv6) == 0 {
		return globalUnicasts[0], nil
	} else if len(globalUnicastsIPv6) > 0 && len(globalUnicasts) == 1 {
		return globalUnicasts[0] + "," + globalUnicastsIPv6[0], nil
	} else if len(globalUnicastsIPv6) > 0 {
		return globalUnicastsIPv6[0], nil
	}

	return "", fmt.Errorf("can't find ip for interface %s", ifaceName)
}

type multiListener struct {
	listeners []net.Listener
	closing   chan struct{}
	conns     chan acceptRes
}

type acceptRes struct {
	conn net.Conn
	err  error
}

// explicit interface check
var _ net.Listener = &multiListener{}

var loopbacks = []string{"127.0.0.1", "::1"}

// ListenWithLoopback listens on the given address, as well as on IPv4 and IPv6 loopback addresses.
// If the address is a wildcard, the listener is return unwrapped.
func ListenWithLoopback(ctx context.Context, addr string, port string) (net.Listener, error) {
	lc := &net.ListenConfig{
		KeepAlive: 3 * time.Minute,
		Control:   permitReuse,
	}
	l, err := lc.Listen(ctx, "tcp", net.JoinHostPort(addr, port))
	if err != nil {
		return nil, err
	}

	// If we're listening on a wildcard address, we don't need to wrap with the other loopback addresses
	switch addr {
	case "", "::", "0.0.0.0":
		return l, nil
	}

	ml := &multiListener{
		listeners: []net.Listener{l},
		closing:   make(chan struct{}),
		conns:     make(chan acceptRes),
	}

	for _, laddr := range loopbacks {
		if laddr == addr {
			continue
		}
		if l, err := lc.Listen(ctx, "tcp", net.JoinHostPort(laddr, port)); err == nil {
			ml.listeners = append(ml.listeners, l)
		} else {
			logrus.Debugf("Failed to listen on %s: %v", net.JoinHostPort(laddr, port), err)
		}
	}

	for i := range ml.listeners {
		go ml.accept(ml.listeners[i])
	}

	return ml, nil
}

// Addr returns the address of the non-loopback address that this multiListener is listening on
func (ml *multiListener) Addr() net.Addr {
	return ml.listeners[0].Addr()
}

// Close closes all the listeners
func (ml *multiListener) Close() error {
	close(ml.closing)
	var errs merr.Errors
	for i := range ml.listeners {
		err := ml.listeners[i].Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return merr.NewErrors(errs)
}

// Accept returns a Conn/err pair from one of the waiting listeners
func (ml *multiListener) Accept() (net.Conn, error) {
	select {
	case res, ok := <-ml.conns:
		if ok {
			return res.conn, res.err
		}
		return nil, fmt.Errorf("connection channel closed")
	case <-ml.closing:
		return nil, fmt.Errorf("listener closed")
	}
}

// accept runs a loop, accepting connections and trying to send on the result channel
func (ml *multiListener) accept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		r := acceptRes{
			conn: conn,
			err:  err,
		}
		select {
		case ml.conns <- r:
		case <-ml.closing:
			if r.err == nil {
				r.conn.Close()
			}
			return
		}
	}
}
