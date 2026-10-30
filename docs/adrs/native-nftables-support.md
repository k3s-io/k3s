# Support native nftables across K3s networking

Date: 2026-10-29

## Status

Proposed

## Context

Linux distributions are moving away from iptables in favor of native nftables (through the `nft` binary).
Several components of k3s manage their own rules, and most of them offer native nftables support even though they are still defaulting to iptables.

The goal is to add native nftables to k3s, instead of relying on `iptables-nft`. The latter translates iptables
commands into nftables rules, but still depends on the iptables tooling and rule model.

This is [already available](https://docs.rke2.io/networking/basic_network_options?_highlight=nftable#nftables-support) in rke2 since the July 2026 releases.

Here is where things stand:

| Component | Current K3s behavior | What needs to change |
| --- | --- | --- |
| kube-proxy | Runs embedded, with `proxy-mode=iptables` by default. | Already configurable with `--kube-proxy-arg=proxy-mode=nftables` on each node. |
| kube-router network-policy controller | Runs embedded with `UseNftablesForNetpol=false`, an ipset precheck, and iptables/ipset handlers. | Select nftables, skip the ipset precheck and iptables handlers, and create and pass the nftables interfaces. The bundled library already supports this. |
| Flannel | Runs embedded with a hardcoded `IPTablesManager`. | Select the existing `NFTablesManager` when the network config has `EnableNFTables=true`. Currently even a custom `--flannel-conf` cannot enable it. |
| CNI portmap | The generated CNI conflist leaves the backend unset, normally selecting iptables. | Generate `"backend": "nftables"`. This already works through a complete custom `--flannel-cni-conf` and bypasses backend auto-detection. |
| ServiceLB / klipper-lb | Uses hostPorts plus iptables rules inside the load-balancer Pods. Its "nft mode" selects `iptables-nft`. | Add native nftables support to klipper-lb, or require ServiceLB to be disabled and replaced. Changing portmap alone is not enough. |

Flannel's `--flannel-backend` selects VXLAN, host-gw, or another network backend;
it does not select the firewall implementation. With the default CNI config,
Flannel handles masquerading, so the bridge plugin does not need a separate
masquerading backend change.

Traefik does not need a firewall backend setting, but its default LoadBalancer
Service depends on ServiceLB. Host firewalls and third-party networking
components are outside this switch.

## Proposal

Add an opt-in native nftables mode for the built-in Linux networking stack.
Keep iptables as the default. Since most of the components run embedded in k3s,
this should be configured globally in the k3s configuration file.
The option name could be `enable-nftables (bool)` with default value set to false.

The mode should configure all enabled components above, not just kube-proxy.
Native nftables support should be added to klipper-lb first. Explicitly
conflicting component arguments or custom configs should produce a clear error.

Use the existing Flannel and kube-router library implementations. Follow their
startup paths for backend initialization and cleanup of the inactive backend.
Missing nftables support must fail clearly, especially for network-policy
enforcement; do not silently fall back to iptables or skip the controller.

Update install, killall, and uninstall cleanup too. The current scripts use
iptables save/restore and will not remove native nftables tables. Cleanup must
target only tables owned by K3s networking components, never flush the host's
whole ruleset.

## Consequences

* Nodes need a compatible kernel and `nft` executable. Kube-proxy requires Linux
  5.13 or later, and the bundled knftables library requires `nft` 1.0.1 or later.
  Packaging and startup checks need to cover these dependencies.
* Migration includes existing rules and Pod sandboxes, not just a restart with
  a different setting. Document Pod recreation, old hostPort rule cleanup, and
  rollback; do not promise a disruption-free switch.
* NodePort address defaults and host-firewall interaction differ in kube-proxy's
  nftables mode. Document those changes rather than assuming identical behavior.
* Cover IPv4, IPv6, dual-stack, NetworkPolicies, Service traffic, hostPorts,
  masquerading, restart, migration, rollback, and uninstall. Check that native
  mode does not leave active iptables-based rules from the enabled components.

## References

* [Embedded network-policy controller](../../pkg/agent/netpol/netpol.go)
* [Embedded Flannel](../../pkg/agent/flannel/flannel.go)
* [Default CNI conflist](../../pkg/agent/flannel/setup_linux.go)
* [kube-proxy defaults](../../pkg/daemons/agent/agent_linux.go)
* [klipper-lb entry script](https://github.com/k3s-io/klipper-lb/blob/v0.4.17/entry)
* [kube-proxy nftables migration notes](https://kubernetes.io/docs/reference/networking/virtual-ips/#migrating-from-iptables-mode-to-nftables)

## Decision

Pending
