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

package ip

import (
	"errors"
	"fmt"
	"github.com/flannel-io/flannel/pkg/powershell"
	"net"
)

// GetInterfaceIP4Addr returns the IPv4 address for the given network interface
func GetInterfaceIP4Addr(iface *net.Interface) (net.IP, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPAddr:
			ip = v.IP
		case *net.IPNet:
			ip = v.IP
		}

		if ip != nil && ip.To4() != nil {
			return ip, nil
		}
	}

	return nil, errors.New("no IPv4 address found for given interface")
}

// GetDefaultGatewayInterface returns the first network interface found with a default gateway set
func GetDefaultGatewayInterface() (*net.Interface, error) {
	index, err := getDefaultGatewayInterfaceIndex()
	if err != nil {
		return nil, err
	}

	return net.InterfaceByIndex(index)
}

func getDefaultGatewayInterfaceIndex() (int, error) {
	powerShellJsonData := struct {
		IfIndex int `json:"ifIndex"`
	}{-1}

	err := powershell.RunCommandWithJsonResult("Get-NetRoute | Where { $_.DestinationPrefix -eq '0.0.0.0/0' } | Select-Object -Property ifIndex", &powerShellJsonData)
	if err != nil {
		return -1, err
	}

	if powerShellJsonData.IfIndex < 0 {
		return -1, errors.New("unable to find default gateway interface index")
	}

	return powerShellJsonData.IfIndex, nil
}

// GetInterfaceByIP tries to get the network interface with the given ip address
func GetInterfaceByIP(search net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.Equal(search) {
				return &i, nil
			}
		}
	}

	return nil, errors.New("no interface with given IP found")
}

// EnableForwardingForInterface enables forwarding for given interface.
// The process must run with elevated rights. Otherwise the function will fail with an "Access Denied" error.
func EnableForwardingForInterface(iface *net.Interface) error {
	return setForwardingForInterface(iface, true)
}

// DisableForwardingForInterface disables forwarding for given interface.
// The process must run with elevated rights. Otherwise the function will fail with an "Access Denied" error.
func DisableForwardingForInterface(iface *net.Interface) error {
	return setForwardingForInterface(iface, false)
}

func setForwardingForInterface(iface *net.Interface, forwarding bool) error {
	value := "Enabled"
	if !forwarding {
		value = "Disabled"
	}

	_, err := powershell.RunCommandf("Set-NetIPInterface -ifIndex %d -AddressFamily IPv4 -Forwarding %s", iface.Index, value)
	if err != nil {
		return err
	}

	return nil
}

func IsForwardingEnabledForInterface(iface *net.Interface) (bool, error) {
	powerShellJsonData := struct {
		Forwarding int `json:"Forwarding"`
	}{0}

	err := powershell.RunCommandWithJsonResult(fmt.Sprintf("Get-NetIPInterface -ifIndex %d -AddressFamily IPv4 | Select-Object -Property Forwarding", iface.Index), &powerShellJsonData)
	if err != nil {
		return false, err
	}

	return powerShellJsonData.Forwarding == 1, nil
}
