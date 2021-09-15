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
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"net"
)

type IP6 big.Int

func FromIP16Bytes(ip []byte) *IP6 {
	return (*IP6)(big.NewInt(0).SetBytes(ip))
}

func FromIP6(ip net.IP) *IP6 {
	ipv6 := ip.To16()

	if ipv6 == nil {
		panic("Address is not an IPv6 address")
	}

	return FromIP16Bytes(ipv6)
}

func ParseIP6(s string) (*IP6, error) {
	ip := net.ParseIP(s)
	if ip == nil {
		return (*IP6)(big.NewInt(0)), errors.New("Invalid IP address format")
	}
	return FromIP6(ip), nil
}

func Mask(prefixLen int) *big.Int {
	mask := net.CIDRMask(prefixLen, 128)
	return big.NewInt(0).SetBytes(mask)
}

func IsEmpty(subnet *IP6) bool {
	if subnet == nil || (*big.Int)(subnet).Cmp(big.NewInt(0)) == 0 {
		return true
	}
	return false
}

func GetIPv6SubnetMin(networkIP *IP6, subnetSize *big.Int) *IP6 {
	return (*IP6)(big.NewInt(0).Add((*big.Int)(networkIP), subnetSize))
}

func GetIPv6SubnetMax(networkIP *IP6, subnetSize *big.Int) *IP6 {
	return (*IP6)(big.NewInt(0).Sub((*big.Int)(networkIP), subnetSize))
}

func CheckIPv6Subnet(subnetIP *IP6, mask *big.Int) bool {
	if (*big.Int)(subnetIP).Cmp(big.NewInt(0).And((*big.Int)(subnetIP), mask)) != 0 {
		return false
	}
	return true
}

func MustParseIP6(s string) *IP6 {
	ip, err := ParseIP6(s)
	if err != nil {
		panic(err)
	}
	return ip
}

func (ip6 *IP6) ToIP() net.IP {
	ip := net.IP((*big.Int)(ip6).Bytes())
	if ip.To4() != nil {
		return ip
	}
	a := (*big.Int)(ip6).FillBytes(make([]byte, 16))
	return a
}

func (ip6 IP6) String() string {
	return ip6.ToIP().String()
}

// MarshalJSON: json.Marshaler impl
func (ip6 IP6) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, ip6)), nil
}

// UnmarshalJSON: json.Unmarshaler impl
func (ip6 *IP6) UnmarshalJSON(j []byte) error {
	j = bytes.Trim(j, "\"")
	if val, err := ParseIP6(string(j)); err != nil {
		return err
	} else {
		*ip6 = *val
		return nil
	}
}

// similar to net.IPNet but has uint based representation
type IP6Net struct {
	IP        *IP6
	PrefixLen uint
}

func (n IP6Net) String() string {
	if n.IP == nil {
		n.IP = (*IP6)(big.NewInt(0))
	}
	return fmt.Sprintf("%s/%d", n.IP.String(), n.PrefixLen)
}

func (n IP6Net) StringSep(hexSep, prefixSep string) string {
	return fmt.Sprintf("%s%s%d", n.IP.String(), prefixSep, n.PrefixLen)
}

func (n IP6Net) Network() IP6Net {
	mask := net.CIDRMask(int(n.PrefixLen), 128)
	return IP6Net{
		FromIP6(n.IP.ToIP().Mask(mask)),
		n.PrefixLen,
	}
}

func (n IP6Net) Next() IP6Net {
	return IP6Net{
		(*IP6)(big.NewInt(0).Add((*big.Int)(n.IP), big.NewInt(0).Lsh(big.NewInt(1), 128-n.PrefixLen))),
		n.PrefixLen,
	}
}

// IncrementIP() increments the IP of IP6Net CIDR by 1
func (n *IP6Net) IncrementIP() {
	n.IP = (*IP6)(big.NewInt(0).Add((*big.Int)(n.IP), big.NewInt(1)))
}

func FromIP6Net(n *net.IPNet) IP6Net {
	prefixLen, _ := n.Mask.Size()
	return IP6Net{
		FromIP6(n.IP),
		uint(prefixLen),
	}
}

func (n IP6Net) ToIPNet() *net.IPNet {
	return &net.IPNet{
		IP:   n.IP.ToIP(),
		Mask: net.CIDRMask(int(n.PrefixLen), 128),
	}
}

func (n IP6Net) Overlaps(other IP6Net) bool {
	var mask *big.Int
	if n.PrefixLen < other.PrefixLen {
		mask = n.Mask()
	} else {
		mask = other.Mask()
	}
	return (IP6)(*big.NewInt(0).And((*big.Int)(n.IP), mask)).String() ==
		(IP6)(*big.NewInt(0).And((*big.Int)(other.IP), mask)).String()
}

func (n IP6Net) Equal(other IP6Net) bool {
	return ((*big.Int)(n.IP).Cmp((*big.Int)(other.IP)) == 0) &&
		n.PrefixLen == other.PrefixLen
}

func (n IP6Net) Mask() *big.Int {
	mask := net.CIDRMask(int(n.PrefixLen), 128)
	return big.NewInt(0).SetBytes(mask)
}

func (n IP6Net) Contains(ip *IP6) bool {
	network := big.NewInt(0).And((*big.Int)(n.IP), n.Mask())
	subnet := big.NewInt(0).And((*big.Int)(ip), n.Mask())
	return (IP6)(*network).String() == (IP6)(*subnet).String()
}

func (n IP6Net) Empty() bool {
	return n.IP == (*IP6)(big.NewInt(0)) && n.PrefixLen == uint(0)
}

// MarshalJSON: json.Marshaler impl
func (n IP6Net) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, n)), nil
}

// UnmarshalJSON: json.Unmarshaler impl
func (n *IP6Net) UnmarshalJSON(j []byte) error {
	j = bytes.Trim(j, "\"")
	if _, val, err := net.ParseCIDR(string(j)); err != nil {
		return err
	} else {
		*n = FromIP6Net(val)
		return nil
	}
}
