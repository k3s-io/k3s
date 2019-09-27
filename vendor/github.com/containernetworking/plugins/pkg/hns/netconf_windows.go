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

package hns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/buger/jsonparser"
	"github.com/containernetworking/cni/pkg/types"
	"strings"
)

// NetConf is the CNI spec
type NetConf struct {
	types.NetConf
	// ApiVersion is either 1 or 2, which specifies which hns APIs to call
	ApiVersion int `json:"ApiVersion"`
	// V2 Api Policies
	HcnPolicyArgs []hcn.EndpointPolicy `json:"HcnPolicyArgs,omitempty"`
	// V1 Api Policies
	Policies []policy `json:"policies,omitempty"`
	// Options to be passed in by the runtime
	RuntimeConfig RuntimeConfig `json:"runtimeConfig"`
	// If true, adds a policy to endpoints to support loopback direct server return
	LoopbackDSR bool `json:"loopbackDSR"`
}

type RuntimeDNS struct {
	Nameservers []string `json:"servers,omitempty"`
	Search      []string `json:"searches,omitempty"`
}

type RuntimeConfig struct {
	DNS RuntimeDNS `json:"dns"`
}

type policy struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

func GetDefaultDestinationPrefix(ip *net.IP) string {
	destinationPrefix := "0.0.0.0/0"
	if ipv6 := ip.To4(); ipv6 == nil {
		destinationPrefix = "::/0"
	}
	return destinationPrefix
}

func (n *NetConf) ApplyLoopbackDSR(ip *net.IP) {
	value := fmt.Sprintf(`"Destinations" : ["%s"]`, ip.String())
	if n.ApiVersion == 2 {
		hcnLoopbackRoute := hcn.EndpointPolicy{
			Type:     "OutBoundNAT",
			Settings: []byte(fmt.Sprintf("{%s}", value)),
		}
		n.HcnPolicyArgs = append(n.HcnPolicyArgs, hcnLoopbackRoute)
	} else {
		hnsLoopbackRoute := policy{
			Name:  "EndpointPolicy",
			Value: []byte(fmt.Sprintf(`{"Type": "OutBoundNAT", %s}`, value)),
		}
		n.Policies = append(n.Policies, hnsLoopbackRoute)
	}
}

// If runtime dns values are there use that else use cni conf supplied dns
func (n *NetConf) GetDNS() types.DNS {
	dnsResult := n.DNS
	if len(n.RuntimeConfig.DNS.Nameservers) > 0 {
		dnsResult.Nameservers = n.RuntimeConfig.DNS.Nameservers
	}
	if len(n.RuntimeConfig.DNS.Search) > 0 {
		dnsResult.Search = n.RuntimeConfig.DNS.Search
	}
	return dnsResult
}

// MarshalPolicies converts the Endpoint policies in Policies
// to HNS specific policies as Json raw bytes
func (n *NetConf) MarshalPolicies() []json.RawMessage {
	if n.Policies == nil {
		n.Policies = make([]policy, 0)
	}

	result := make([]json.RawMessage, 0, len(n.Policies))
	for _, p := range n.Policies {
		if !strings.EqualFold(p.Name, "EndpointPolicy") {
			continue
		}

		result = append(result, p.Value)
	}

	return result
}

// ApplyOutboundNatPolicy applies NAT Policy in VFP using HNS
// Simultaneously an exception is added for the network that has to be Nat'd
func (n *NetConf) ApplyOutboundNatPolicy(nwToNat string) {
	if n.Policies == nil {
		n.Policies = make([]policy, 0)
	}

	nwToNatBytes := []byte(nwToNat)

	for i, p := range n.Policies {
		if !strings.EqualFold(p.Name, "EndpointPolicy") {
			continue
		}

		typeValue, err := jsonparser.GetUnsafeString(p.Value, "Type")
		if err != nil || len(typeValue) == 0 {
			continue
		}

		if !strings.EqualFold(typeValue, "OutBoundNAT") {
			continue
		}

		exceptionListValue, dt, _, _ := jsonparser.Get(p.Value, "ExceptionList")
		// OutBoundNAT must with ExceptionList, so don't need to judge jsonparser.NotExist
		if dt == jsonparser.Array {
			buf := bytes.Buffer{}
			buf.WriteString(`{"Type": "OutBoundNAT", "ExceptionList": [`)

			jsonparser.ArrayEach(exceptionListValue, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
				if dataType == jsonparser.String && len(value) != 0 {
					if bytes.Compare(value, nwToNatBytes) != 0 {
						buf.WriteByte('"')
						buf.Write(value)
						buf.WriteByte('"')
						buf.WriteByte(',')
					}
				}
			})

			buf.WriteString(`"` + nwToNat + `"]}`)

			n.Policies[i] = policy{
				Name:  "EndpointPolicy",
				Value: buf.Bytes(),
			}
		} else {
			n.Policies[i] = policy{
				Name:  "EndpointPolicy",
				Value: []byte(`{"Type": "OutBoundNAT", "ExceptionList": ["` + nwToNat + `"]}`),
			}
		}

		return
	}

	// didn't find the policyArg, add it
	n.Policies = append(n.Policies, policy{
		Name:  "EndpointPolicy",
		Value: []byte(`{"Type": "OutBoundNAT", "ExceptionList": ["` + nwToNat + `"]}`),
	})
}

// ApplyDefaultPAPolicy is used to configure a endpoint PA policy in HNS
func (n *NetConf) ApplyDefaultPAPolicy(paAddress string) {
	if n.Policies == nil {
		n.Policies = make([]policy, 0)
	}

	// if its already present, leave untouched
	for i, p := range n.Policies {
		if !strings.EqualFold(p.Name, "EndpointPolicy") {
			continue
		}

		paValue, dt, _, _ := jsonparser.Get(p.Value, "PA")
		if dt == jsonparser.NotExist {
			continue
		} else if dt == jsonparser.String && len(paValue) != 0 {
			// found it, don't override
			return
		}

		n.Policies[i] = policy{
			Name:  "EndpointPolicy",
			Value: []byte(`{"Type": "PA", "PA": "` + paAddress + `"}`),
		}
		return
	}

	// didn't find the policyArg, add it
	n.Policies = append(n.Policies, policy{
		Name:  "EndpointPolicy",
		Value: []byte(`{"Type": "PA", "PA": "` + paAddress + `"}`),
	})
}
