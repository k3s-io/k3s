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

package subnet

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coreos/flannel/pkg/ip"
)

type Config struct {
	Network     ip.IP4Net
	SubnetMin   ip.IP4
	SubnetMax   ip.IP4
	SubnetLen   uint
	BackendType string          `json:"-"`
	Backend     json.RawMessage `json:",omitempty"`
}

func parseBackendType(be json.RawMessage) (string, error) {
	var bt struct {
		Type string
	}

	if len(be) == 0 {
		return "udp", nil
	} else if err := json.Unmarshal(be, &bt); err != nil {
		return "", fmt.Errorf("error decoding Backend property of config: %v", err)
	}

	return bt.Type, nil
}

func ParseConfig(s string) (*Config, error) {
	cfg := new(Config)
	err := json.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	if cfg.SubnetLen > 0 {
		// SubnetLen needs to allow for a tunnel and bridge device on each host.
		if cfg.SubnetLen > 30 {
			return nil, errors.New("SubnetLen must be less than /31")
		}

		// SubnetLen needs to fit _more_ than twice into the Network.
		// the first subnet isn't used, so splitting into two one only provide one usable host.
		if cfg.SubnetLen < cfg.Network.PrefixLen+2 {
			return nil, errors.New("Network must be able to accommodate at least four subnets")
		}
	} else {
		// If the network is smaller than a /28 then the network isn't big enough for flannel so return an error.
		// Default to giving each host at least a /24 (as long as the network is big enough to support at least four hosts)
		// Otherwise, if the network is too small to give each host a /24 just split the network into four.
		if cfg.Network.PrefixLen > 28 {
			// Each subnet needs at least four addresses (/30) and the network needs to accommodate at least four
			// since the first subnet isn't used, so splitting into two would only provide one usable host.
			// So the min useful PrefixLen is /28
			return nil, errors.New("Network is too small. Minimum useful network prefix is /28")
		} else if cfg.Network.PrefixLen <= 22 {
			// Network is big enough to give each host a /24
			cfg.SubnetLen = 24
		} else {
			// Use +2 to provide four hosts per subnet.
			cfg.SubnetLen = cfg.Network.PrefixLen + 2
		}
	}

	subnetSize := ip.IP4(1 << (32 - cfg.SubnetLen))

	if cfg.SubnetMin == ip.IP4(0) {
		// skip over the first subnet otherwise it causes problems. e.g.
		// if Network is 10.100.0.0/16, having an interface with 10.0.0.0
		// conflicts with the broadcast address.
		cfg.SubnetMin = cfg.Network.IP + subnetSize
	} else if !cfg.Network.Contains(cfg.SubnetMin) {
		return nil, errors.New("SubnetMin is not in the range of the Network")
	}

	if cfg.SubnetMax == ip.IP4(0) {
		cfg.SubnetMax = cfg.Network.Next().IP - subnetSize
	} else if !cfg.Network.Contains(cfg.SubnetMax) {
		return nil, errors.New("SubnetMax is not in the range of the Network")
	}

	// The SubnetMin and SubnetMax need to be aligned to a SubnetLen boundary
	mask := ip.IP4(0xFFFFFFFF << (32 - cfg.SubnetLen))
	if cfg.SubnetMin != cfg.SubnetMin&mask {
		return nil, fmt.Errorf("SubnetMin is not on a SubnetLen boundary: %v", cfg.SubnetMin)
	}

	if cfg.SubnetMax != cfg.SubnetMax&mask {
		return nil, fmt.Errorf("SubnetMax is not on a SubnetLen boundary: %v", cfg.SubnetMax)
	}

	bt, err := parseBackendType(cfg.Backend)
	if err != nil {
		return nil, err
	}
	cfg.BackendType = bt

	return cfg, nil
}
