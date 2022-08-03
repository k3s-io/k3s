package netutil

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func GetIPFromInterface(ifaceName string) string {
	ip, err := getIPFromInterface(ifaceName)
	if err != nil {
		logrus.Warn(errors.Wrap(err, "unable to get global unicast ip from interface name"))
	} else {
		logrus.Infof("Found ip %s from iface %s", ip, ifaceName)
	}
	return ip
}

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
			return "", errors.Wrapf(err, "unable to parse CIDR for interface %s", iface.Name)
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
