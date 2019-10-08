package netroute

import (
	"regexp"
	"net"
	"strconv"
	"strings"
	"bufio"
	"bytes"
	ps "github.com/bhendo/go-powershell"
	psbe "github.com/bhendo/go-powershell/backend"

	"fmt"
	"math/big"
)

// Interface is an injectable interface for running MSFT_NetRoute commands. Implementations must be goroutine-safe.
type Interface interface {
	// Get all net routes on the host
	GetNetRoutesAll() ([]Route, error)

	// Get net routes by link and destination subnet
	GetNetRoutes(linkIndex int, destinationSubnet *net.IPNet) ([]Route, error)

	// Create a new route
	NewNetRoute(linkIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) error

	// Remove an existing route
	RemoveNetRoute(linkIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) error

	// exit the shell
	Exit()
}

type Route struct {
	LinkIndex         int
	DestinationSubnet *net.IPNet
	GatewayAddress    net.IP
	RouteMetric       int
	IfMetric          int
}

type shell struct {
	shellInstance ps.Shell
}

func New() Interface {

	s, _ := ps.New(&psbe.Local{})

	runner := &shell{
		shellInstance: s,
	}

	return runner
}

func (shell *shell) Exit() {
	shell.shellInstance.Exit()
	shell.shellInstance = nil
}

func (shell *shell) GetNetRoutesAll() ([]Route, error) {
	getRouteCmdLine := "get-netroute -erroraction Ignore"
	stdout, err := shell.runScript(getRouteCmdLine)
	if err != nil {
		return nil, err
	}
	return parseRoutesList(stdout), nil
}
func (shell *shell) GetNetRoutes(linkIndex int, destinationSubnet *net.IPNet) ([]Route, error) {
	getRouteCmdLine := fmt.Sprintf("get-netroute -InterfaceIndex %v -DestinationPrefix %v -erroraction Ignore", linkIndex, destinationSubnet.String())
	stdout, err := shell.runScript(getRouteCmdLine)
	if err != nil {
		return nil, err
	}
	return parseRoutesList(stdout), nil
}

func (shell *shell) RemoveNetRoute(linkIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) error {
	removeRouteCmdLine := fmt.Sprintf("remove-netroute -InterfaceIndex %v -DestinationPrefix %v -NextHop  %v -Verbose -Confirm:$false", linkIndex, destinationSubnet.String(), gatewayAddress.String())
	_, err := shell.runScript(removeRouteCmdLine)

	return err
}

func (shell *shell) NewNetRoute(linkIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) error {
	newRouteCmdLine := fmt.Sprintf("new-netroute -InterfaceIndex %v -DestinationPrefix %v -NextHop  %v -Verbose", linkIndex, destinationSubnet.String(), gatewayAddress.String())
	_, err := shell.runScript(newRouteCmdLine)

	return err
}

func parseRoutesList(stdout string) []Route {
	internalWhitespaceRegEx := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	var routes []Route
	for scanner.Scan() {
		line := internalWhitespaceRegEx.ReplaceAllString(scanner.Text(), "|")
		if strings.HasPrefix(line, "ifIndex") || strings.HasPrefix(line, "----") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			continue
		}

		linkIndex, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		gatewayAddress := net.ParseIP(parts[2])
		if gatewayAddress == nil {
			continue
		}

		_, destinationSubnet, err := net.ParseCIDR(parts[1])
		if err != nil {
			continue
		}
		route := Route{
			DestinationSubnet: destinationSubnet,
			GatewayAddress:    gatewayAddress,
			LinkIndex:         linkIndex,
		}

		routes = append(routes, route)
	}

	return routes
}

func (r *Route) Equal(route Route) bool {
	if r.DestinationSubnet.IP.Equal(route.DestinationSubnet.IP) && r.GatewayAddress.Equal(route.GatewayAddress) && bytes.Equal(r.DestinationSubnet.Mask, route.DestinationSubnet.Mask) {
		return true
	}

	return false
}

func (shell *shell) runScript(cmdLine string) (string, error) {

	stdout, _, err := shell.shellInstance.Execute(cmdLine)
	if err != nil {
		return "", err
	}

	return stdout, nil
}

func IpToInt(ip net.IP) *big.Int {
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v)
	}
	return big.NewInt(0).SetBytes(ip.To16())
}

func IntToIP(i *big.Int) net.IP {
	return net.IP(i.Bytes())
}
