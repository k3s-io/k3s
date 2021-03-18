// +build windows

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

package routing

import (
	"fmt"
	"github.com/flannel-io/flannel/pkg/powershell"
	"net"
)

// Router manages network routes on Windows OS using MSFT_NetRoute
// See also https://docs.microsoft.com/en-us/previous-versions/windows/desktop/legacy/hh872448(v%3Dvs.85)
type RouterWindows struct{}

func (r RouterWindows) GetAllRoutes() ([]Route, error) {
	return parseNetRoutes("@(Get-NetRoute | Select-Object -Property IfIndex,DestinationPrefix,NextHop)")
}

func (r RouterWindows) GetRoutesFromInterfaceToSubnet(interfaceIndex int, destinationSubnet *net.IPNet) ([]Route, error) {
	return parseNetRoutes(fmt.Sprintf("@(Get-NetRoute -InterfaceIndex %d -DestinationPrefix %s | Select-Object -Property IfIndex,DestinationPrefix,NextHop)", interfaceIndex, destinationSubnet.String()))
}

func (r RouterWindows) CreateRoute(interfaceIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) error {
	_, err := powershell.RunCommandf("New-NetRoute -InterfaceIndex %d -DestinationPrefix %s -NextHop  %s", interfaceIndex, destinationSubnet.String(), gatewayAddress.String())
	return err
}

func (r RouterWindows) DeleteRoute(interfaceIndex int, destinationSubnet *net.IPNet, gatewayAddress net.IP) error {
	_, err := powershell.RunCommandf("Remove-NetRoute -InterfaceIndex %d -DestinationPrefix %s -NextHop %s -Verbose -Confirm:$false", interfaceIndex, destinationSubnet.String(), gatewayAddress.String())
	return err
}

type winNetRoute struct {
	IfIndex           int
	DestinationPrefix string
	NextHop           string
}

func parseNetRoutes(cmd string) ([]Route, error) {
	powerShellJsonData := make([]winNetRoute, 0)

	err := powershell.RunCommandWithJsonResult(cmd, &powerShellJsonData)
	if err != nil {
		return nil, err
	}

	routes := make([]Route, 0)
	for _, r := range powerShellJsonData {
		route := Route{
			InterfaceIndex: r.IfIndex,
		}

		_, destinationSubnet, err := net.ParseCIDR(r.DestinationPrefix)
		if err != nil {
			continue
		}
		route.DestinationSubnet = destinationSubnet

		gatewayAddress := net.ParseIP(r.NextHop)
		if gatewayAddress == nil {
			continue
		}
		route.GatewayAddress = gatewayAddress

		routes = append(routes, route)
	}

	return routes, nil
}
