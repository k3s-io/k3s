// Copyright 2017 CNI authors
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

// This is a post-setup plugin that establishes port forwarding - using iptables,
// from the host's network interface(s) to a pod's network interface.
//
// It is intended to be used as a chained CNI plugin, and determines the container
// IP from the previous result. If the result includes an IPv6 address, it will
// also be configured. (IPTables will not forward cross-family).
//
// This has one notable limitation: it does not perform any kind of reservation
// of the actual host port. If there is a service on the host, it will have all
// its traffic captured by the container. If another container also claims a given
// port, it will caputure the traffic - it is last-write-wins.
package portmap

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"

	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
)

// PortMapEntry corresponds to a single entry in the port_mappings argument,
// see CONVENTIONS.md
type PortMapEntry struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIP,omitempty"`
}

type PortMapConf struct {
	types.NetConf
	SNAT                 *bool     `json:"snat,omitempty"`
	ConditionsV4         *[]string `json:"conditionsV4"`
	ConditionsV6         *[]string `json:"conditionsV6"`
	MarkMasqBit          *int      `json:"markMasqBit"`
	ExternalSetMarkChain *string   `json:"externalSetMarkChain"`
	RuntimeConfig        struct {
		PortMaps []PortMapEntry `json:"portMappings,omitempty"`
	} `json:"runtimeConfig,omitempty"`

	// These are fields parsed out of the config or the environment;
	// included here for convenience
	ContainerID string `json:"-"`
	ContIPv4    net.IP `json:"-"`
	ContIPv6    net.IP `json:"-"`
}

// The default mark bit to signal that masquerading is required
// Kubernetes uses 14 and 15, Calico uses 20-31.
const DefaultMarkBit = 13

func cmdAdd(args *skel.CmdArgs) error {
	netConf, _, err := parseConfig(args.StdinData, args.IfName)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	if netConf.PrevResult == nil {
		return fmt.Errorf("must be called as chained plugin")
	}

	if len(netConf.RuntimeConfig.PortMaps) == 0 {
		return types.PrintResult(netConf.PrevResult, netConf.CNIVersion)
	}

	netConf.ContainerID = args.ContainerID

	if netConf.ContIPv4 != nil {
		if err := forwardPorts(netConf, netConf.ContIPv4); err != nil {
			return err
		}
	}

	if netConf.ContIPv6 != nil {
		if err := forwardPorts(netConf, netConf.ContIPv6); err != nil {
			return err
		}
	}

	// Pass through the previous result
	return types.PrintResult(netConf.PrevResult, netConf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	netConf, _, err := parseConfig(args.StdinData, args.IfName)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	netConf.ContainerID = args.ContainerID

	// We don't need to parse out whether or not we're using v6 or snat,
	// deletion is idempotent
	if err := unforwardPorts(netConf); err != nil {
		return err
	}
	return nil
}

func Main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("portmap"))
}

func cmdCheck(args *skel.CmdArgs) error {
	conf, result, err := parseConfig(args.StdinData, args.IfName)
	if err != nil {
		return err
	}

	// Ensure we have previous result.
	if result == nil {
		return fmt.Errorf("Required prevResult missing")
	}

	if len(conf.RuntimeConfig.PortMaps) == 0 {
		return nil
	}

	conf.ContainerID = args.ContainerID

	if conf.ContIPv4 != nil {
		if err := checkPorts(conf, conf.ContIPv4); err != nil {
			return err
		}
	}

	if conf.ContIPv6 != nil {
		if err := checkPorts(conf, conf.ContIPv6); err != nil {
			return err
		}
	}

	return nil
}

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func parseConfig(stdin []byte, ifName string) (*PortMapConf, *current.Result, error) {
	conf := PortMapConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	// Parse previous result.
	var result *current.Result
	if conf.RawPrevResult != nil {
		var err error
		if err = version.ParsePrevResult(&conf.NetConf); err != nil {
			return nil, nil, fmt.Errorf("could not parse prevResult: %v", err)
		}

		result, err = current.NewResultFromResult(conf.PrevResult)
		if err != nil {
			return nil, nil, fmt.Errorf("could not convert result to current version: %v", err)
		}
	}

	if conf.SNAT == nil {
		tvar := true
		conf.SNAT = &tvar
	}

	if conf.MarkMasqBit != nil && conf.ExternalSetMarkChain != nil {
		return nil, nil, fmt.Errorf("Cannot specify externalSetMarkChain and markMasqBit")
	}

	if conf.MarkMasqBit == nil {
		bvar := DefaultMarkBit // go constants are "special"
		conf.MarkMasqBit = &bvar
	}

	if *conf.MarkMasqBit < 0 || *conf.MarkMasqBit > 31 {
		return nil, nil, fmt.Errorf("MasqMarkBit must be between 0 and 31")
	}

	// Reject invalid port numbers
	for _, pm := range conf.RuntimeConfig.PortMaps {
		if pm.ContainerPort <= 0 {
			return nil, nil, fmt.Errorf("Invalid container port number: %d", pm.ContainerPort)
		}
		if pm.HostPort <= 0 {
			return nil, nil, fmt.Errorf("Invalid host port number: %d", pm.HostPort)
		}
	}

	if conf.PrevResult != nil {
		for _, ip := range result.IPs {
			if ip.Version == "6" && conf.ContIPv6 != nil {
				continue
			} else if ip.Version == "4" && conf.ContIPv4 != nil {
				continue
			}

			// Skip known non-sandbox interfaces
			if ip.Interface != nil {
				intIdx := *ip.Interface
				if intIdx >= 0 &&
					intIdx < len(result.Interfaces) &&
					(result.Interfaces[intIdx].Name != ifName ||
						result.Interfaces[intIdx].Sandbox == "") {
					continue
				}
			}
			switch ip.Version {
			case "6":
				conf.ContIPv6 = ip.Address.IP
			case "4":
				conf.ContIPv4 = ip.Address.IP
			}
		}
	}

	return &conf, result, nil
}
