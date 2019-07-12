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

package portmap

import (
	"fmt"
	"net"
	"sort"
	"strconv"

	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/coreos/go-iptables/iptables"
)

// This creates the chains to be added to iptables. The basic structure is
// a bit complex for efficiency's sake. We create 2 chains: a summary chain
// that is shared between invocations, and an invocation (container)-specific
// chain. This minimizes the number of operations on the top level, but allows
// for easy cleanup.
//
// The basic setup (all operations are on the nat table) is:
//
// DNAT case (rewrite destination IP and port):
// PREROUTING, OUTPUT: --dst-type local -j CNI-HOSTPORT-DNAT
// CNI-HOSTPORT-DNAT: --destination-ports 8080,8081 -j CNI-DN-abcd123
// CNI-DN-abcd123: -p tcp --dport 8080 -j DNAT --to-destination 192.0.2.33:80
// CNI-DN-abcd123: -p tcp --dport 8081 -j DNAT ...

// The names of the top-level summary chains.
// These should never be changed, or else upgrading will require manual
// intervention.
const TopLevelDNATChainName = "CNI-HOSTPORT-DNAT"
const SetMarkChainName = "CNI-HOSTPORT-SETMARK"
const MarkMasqChainName = "CNI-HOSTPORT-MASQ"
const OldTopLevelSNATChainName = "CNI-HOSTPORT-SNAT"

// forwardPorts establishes port forwarding to a given container IP.
// containerIP can be either v4 or v6.
func forwardPorts(config *PortMapConf, containerIP net.IP) error {
	isV6 := (containerIP.To4() == nil)

	var ipt *iptables.IPTables
	var err error

	if isV6 {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv6)
	} else {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
	}
	if err != nil {
		return fmt.Errorf("failed to open iptables: %v", err)
	}

	// Enable masquerading for traffic as necessary.
	// The DNAT chain sets a mark bit for traffic that needs masq:
	// - connections from localhost
	// - hairpin traffic back to the container
	// Idempotently create the rule that masquerades traffic with this mark.
	// Need to do this first; the DNAT rules reference these chains
	if *config.SNAT {
		if config.ExternalSetMarkChain == nil {
			setMarkChain := genSetMarkChain(*config.MarkMasqBit)
			if err := setMarkChain.setup(ipt); err != nil {
				return fmt.Errorf("unable to create chain %s: %v", setMarkChain.name, err)
			}

			masqChain := genMarkMasqChain(*config.MarkMasqBit)
			if err := masqChain.setup(ipt); err != nil {
				return fmt.Errorf("unable to create chain %s: %v", setMarkChain.name, err)
			}
		}

		if !isV6 {
			// Set the route_localnet bit on the host interface, so that
			// 127/8 can cross a routing boundary.
			hostIfName := getRoutableHostIF(containerIP)
			if hostIfName != "" {
				if err := enableLocalnetRouting(hostIfName); err != nil {
					return fmt.Errorf("unable to enable route_localnet: %v", err)
				}
			}
		}
	}

	// Generate the DNAT (actual port forwarding) rules
	toplevelDnatChain := genToplevelDnatChain()
	if err := toplevelDnatChain.setup(ipt); err != nil {
		return fmt.Errorf("failed to create top-level DNAT chain: %v", err)
	}

	dnatChain := genDnatChain(config.Name, config.ContainerID)
	// First, idempotently tear down this chain in case there was some
	// sort of collision or bad state.
	fillDnatRules(&dnatChain, config, containerIP)
	if err := dnatChain.setup(ipt); err != nil {
		return fmt.Errorf("unable to setup DNAT: %v", err)
	}

	return nil
}

// genToplevelDnatChain creates the top-level summary chain that we'll
// add our chain to. This is easy, because creating chains is idempotent.
// IMPORTANT: do not change this, or else upgrading plugins will require
// manual intervention.
func genToplevelDnatChain() chain {
	return chain{
		table: "nat",
		name:  TopLevelDNATChainName,
		entryRules: [][]string{{
			"-m", "addrtype",
			"--dst-type", "LOCAL",
		}},
		entryChains: []string{"PREROUTING", "OUTPUT"},
	}
}

// genDnatChain creates the per-container chain.
// Conditions are any static entry conditions for the chain.
func genDnatChain(netName, containerID string) chain {
	return chain{
		table:       "nat",
		name:        formatChainName("DN-", netName, containerID),
		entryChains: []string{TopLevelDNATChainName},
	}
}

// dnatRules generates the destination NAT rules, one per port, to direct
// traffic from hostip:hostport to podip:podport
func fillDnatRules(c *chain, config *PortMapConf, containerIP net.IP) {
	isV6 := (containerIP.To4() == nil)
	comment := trimComment(fmt.Sprintf(`dnat name: "%s" id: "%s"`, config.Name, config.ContainerID))
	entries := config.RuntimeConfig.PortMaps
	setMarkChainName := SetMarkChainName
	if config.ExternalSetMarkChain != nil {
		setMarkChainName = *config.ExternalSetMarkChain
	}

	//Generate the dnat entry rules. We'll use multiport, but it ony accepts
	// up to 15 rules, so partition the list if needed.
	// Do it in a stable order for testing
	protoPorts := groupByProto(entries)
	protos := []string{}
	for proto := range protoPorts {
		protos = append(protos, proto)
	}
	sort.Strings(protos)
	for _, proto := range protos {
		for _, portSpec := range splitPortList(protoPorts[proto]) {
			r := []string{
				"-m", "comment",
				"--comment", comment,
				"-m", "multiport",
				"-p", proto,
				"--destination-ports", portSpec,
			}

			if isV6 && config.ConditionsV6 != nil && len(*config.ConditionsV6) > 0 {
				r = append(r, *config.ConditionsV6...)
			} else if !isV6 && config.ConditionsV4 != nil && len(*config.ConditionsV4) > 0 {
				r = append(r, *config.ConditionsV4...)
			}
			c.entryRules = append(c.entryRules, r)
		}
	}

	// For every entry, generate 3 rules:
	// - mark hairpin for masq
	// - mark localhost for masq (for v4)
	// - do dnat
	// the ordering is important here; the mark rules must be first.
	c.rules = make([][]string, 0, 3*len(entries))
	for _, entry := range entries {
		ruleBase := []string{
			"-p", entry.Protocol,
			"--dport", strconv.Itoa(entry.HostPort)}
		if entry.HostIP != "" {
			ruleBase = append(ruleBase,
				"-d", entry.HostIP)
		}

		// Add mark-to-masquerade rules for hairpin and localhost
		if *config.SNAT {
			// hairpin
			hpRule := make([]string, len(ruleBase), len(ruleBase)+4)
			copy(hpRule, ruleBase)

			hpRule = append(hpRule,
				"-s", containerIP.String(),
				"-j", setMarkChainName,
			)
			c.rules = append(c.rules, hpRule)

			if !isV6 {
				// localhost
				localRule := make([]string, len(ruleBase), len(ruleBase)+4)
				copy(localRule, ruleBase)

				localRule = append(localRule,
					"-s", "127.0.0.1",
					"-j", setMarkChainName,
				)
				c.rules = append(c.rules, localRule)
			}
		}

		// The actual dnat rule
		dnatRule := make([]string, len(ruleBase), len(ruleBase)+4)
		copy(dnatRule, ruleBase)
		dnatRule = append(dnatRule,
			"-j", "DNAT",
			"--to-destination", fmtIpPort(containerIP, entry.ContainerPort),
		)
		c.rules = append(c.rules, dnatRule)
	}
}

// genSetMarkChain creates the SETMARK chain - the chain that sets the
// "to-be-masqueraded" mark and returns.
// Chains are idempotent, so we'll always create this.
func genSetMarkChain(markBit int) chain {
	markValue := 1 << uint(markBit)
	markDef := fmt.Sprintf("%#x/%#x", markValue, markValue)
	ch := chain{
		table: "nat",
		name:  SetMarkChainName,
		rules: [][]string{{
			"-m", "comment",
			"--comment", "CNI portfwd masquerade mark",
			"-j", "MARK",
			"--set-xmark", markDef,
		}},
	}
	return ch
}

// genMarkMasqChain creates the chain that masquerades all packets marked
// in the SETMARK chain
func genMarkMasqChain(markBit int) chain {
	markValue := 1 << uint(markBit)
	markDef := fmt.Sprintf("%#x/%#x", markValue, markValue)
	ch := chain{
		table:       "nat",
		name:        MarkMasqChainName,
		entryChains: []string{"POSTROUTING"},
		// Only this entry chain needs to be prepended, because otherwise it is
		// stomped on by the masquerading rules created by the CNI ptp and bridge
		// plugins.
		prependEntry: true,
		entryRules: [][]string{{
			"-m", "comment",
			"--comment", "CNI portfwd requiring masquerade",
		}},
		rules: [][]string{{
			"-m", "mark",
			"--mark", markDef,
			"-j", "MASQUERADE",
		}},
	}
	return ch
}

// enableLocalnetRouting tells the kernel not to treat 127/8 as a martian,
// so that connections with a source ip of 127/8 can cross a routing boundary.
func enableLocalnetRouting(ifName string) error {
	routeLocalnetPath := "net.ipv4.conf." + ifName + ".route_localnet"
	_, err := sysctl.Sysctl(routeLocalnetPath, "1")
	return err
}

// genOldSnatChain is no longer used, but used to be created. We'll try and
// tear it down in case the plugin version changed between ADD and DEL
func genOldSnatChain(netName, containerID string) chain {
	name := formatChainName("SN-", netName, containerID)

	return chain{
		table:       "nat",
		name:        name,
		entryChains: []string{OldTopLevelSNATChainName},
	}
}

// unforwardPorts deletes any iptables rules created by this plugin.
// It should be idempotent - it will not error if the chain does not exist.
//
// We also need to be a bit clever about how we handle errors with initializing
// iptables. We may be on a system with no ip(6)tables, or no kernel support
// for that protocol. The ADD would be successful, since it only adds forwarding
// based on the addresses assigned to the container. However, at DELETE time we
// don't know which protocols were used.
// So, we first check that iptables is "generally OK" by doing a check. If
// not, we ignore the error, unless neither v4 nor v6 are OK.
func unforwardPorts(config *PortMapConf) error {
	dnatChain := genDnatChain(config.Name, config.ContainerID)

	// Might be lying around from old versions
	oldSnatChain := genOldSnatChain(config.Name, config.ContainerID)

	ip4t := maybeGetIptables(false)
	ip6t := maybeGetIptables(true)
	if ip4t == nil && ip6t == nil {
		return fmt.Errorf("neither iptables nor ip6tables usable")
	}

	if ip4t != nil {
		if err := dnatChain.teardown(ip4t); err != nil {
			return fmt.Errorf("could not teardown ipv4 dnat: %v", err)
		}
		oldSnatChain.teardown(ip4t)
	}

	if ip6t != nil {
		if err := dnatChain.teardown(ip6t); err != nil {
			return fmt.Errorf("could not teardown ipv6 dnat: %v", err)
		}
		oldSnatChain.teardown(ip6t)
	}
	return nil
}

// maybeGetIptables implements the soft error swallowing. If iptables is
// usable for the given protocol, returns a handle, otherwise nil
func maybeGetIptables(isV6 bool) *iptables.IPTables {
	proto := iptables.ProtocolIPv4
	if isV6 {
		proto = iptables.ProtocolIPv6
	}

	ipt, err := iptables.NewWithProtocol(proto)
	if err != nil {
		return nil
	}

	_, err = ipt.List("nat", "OUTPUT")
	if err != nil {
		return nil
	}

	return ipt
}
