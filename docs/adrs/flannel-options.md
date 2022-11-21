# Record architecture decisions

Date: 2022-12-12

## Status

Discussing

## Context

### Flannel upstream

Flannel is the default and only supported CNI plugin in k3s. Flannel runs in each node and, on a high level, does the following:

1 - Generates a general configuration based on flannel passed flags and flannel subnet&backend configuration (normally passed as configmap)
2 - Based on the previous general configuration, it deploys the required infrastructure (e.g. vxlan VTEP)
3 - Based on the previous general configuration, it yields a flannel CNI configuration file that will be used when creating/deleting pods. Note that CNI plugins read the config file each time Kubelet requests their service.

Kubernetes relevant flannel flags are:

`--iface (string)`: interface to use (IP or name) for inter-host communication. Can be specified multiple times to check each option in order. Returns the first match found.
`--iface-can-reach (string)`: detect interface to use (IP or name) for inter-host communication based on which will be used for provided IP. This is exactly the interface to use of command 'ip route get <ip-address>'
`--iface-regex (string)`: regex expression to match the first interface to use (IP or name) for inter-host communication
`--ip-masq (bool)`: setup IP masquerade rule for traffic destined outside of overlay networki (applies to both IPv4 and IPv6)
`--iptables-forward-rules (bool)`: Overrides default rule of FORWARD chain to ACCEPT
`--public-ip (string)`: IP accessible by other nodes for inter-host communication
`--public-ipv6 (string)`: IPv6 accessible by other nodes for inter-host communication


Kubernetes relevant flannel subnet&backend configuration parameters:

Network (string): IPv4 network in CIDR format to use for the entire flannel network (Mandatory)
IPv6Network (string): IPv6 network in CIDR format to use for the entire flannel network. (Mandatory if EnableIPv6 is true)
EnableIPv6 (bool): Enables ipv6 support
Backend (dictionary): Type of backend to use and specific configurations for that backend.


### K3s using Flannel

K3s prepares both flannel flags and flannel subnet&backend configuration before starting flannel. Some of the mentioned flags and config parameters can be configured using k3s flags, some are hardcoded and some are not supported in the current K3s:

`--iface (string)`: Can be set by using k3s' agent flag: `--flannel-iface`
`--iface-can-reach (string)`: Not supported by k3s **
`--iface-regex (string)`: Not supported by k3s **
`--ip-masq (bool)`: ipv4 hardcoded to true. ipv6 configurable by k3s server flag `--flannel-ipv6-masq `
`--iptables-forward-rules (bool)`: hardcoded to true
`--public-ip (string)`: hardcoded to what is passed as k3s agent flag `--external-ip` when it is of IPv4 nature if k3s server flag `--flannel-external-ip` is true
`--public-ipv6 (string)`: hardcoded to what is passed as k3s agent flag `--external-ip` when it is of IPv6 nature if k3s server flag `--flannel-external-ip` is true

** Does not provide much value as flannel in k3s can configure the interface per node

Regarding subnet&backend configuration:

`Network` and `IPv6Network` are set by reading the assigned podCIDR for the Kubernetes node.
`EnableIPv6` is set based on what the user passed as k3s server `--cluster-cidr`.
`Backend` uses vxlan as default, although this can be changed using k3s server flag `--flannel-backend`

Something important to note is that k3s allows the user to override the whole subnet&backend configuration, by using the k3s agent flag `--flannel-conf` and even the flannel CNI configuration file, by using the k3s agent flag `--flanel-cni-conf`. Overriding the whole subnet&backend configuration is useful if user wants to add specific backend configurations, e.g. VNI for vxlan. Users can currently do this by using `<=option1=val1,option2=val2>` when selecting the backend but it seldom used and complicates things, thus it was deprecated in favor of `--flannel-cni-conf`.

To wrap up the context, k3s includes 3 agent flannel options:

* flannel-iface (string): Overrides the default flannel interface. This interface is used to forward encapsulated traffic in inter-node communication. It matches the flannel flag `--iface`
* flannel-conf (string): Path that points to a file containing the flannel subnet&backend config
* flannel-cni-conf (string): Path that point to a flannel CNI config file. This config sets CNI specific configuration for that node

And 3 server options:

* flannel-backend (string): Sets the encapsulation technology that will allow inter-node traffic to work. It matches part of the Backend parameter in the subnet&backend configuration
* flannel-ipv6-masq (bool): Enables masquerading traffic for IPv6. Note that IPv4 traffic is always masqueraded. It matches the ipv6 part of the `--ip-masq` flag
* flannel-external-ip (bool): Enables using node external IP addresses for flannel traffic. It sort of matches on `--public-ip` and `--public-ipv6` flag. In this case, k3s selects the external-ips for those flags 


### Design suggestion

It was suggested to include a string slice flag that would consolidate all flannel server flags into just one. Benefits:
* Simplify user's life by having less flags
* Use it for potential new flannel server flags

Downsides:
* Users are used to those flags, specially `flannel-backend`, therefore it might be seen as a negative changei. That's why we should carefully follow the deprecation policy.

## Proposal

Starting in v1.26, introduce a new `flannel-opts` flag that includes flannel server options. The redundant flags are deprecated and removed in a few releases.

We could reduce it to 2:

* flannel-backend (string)
* flannel-opts ([]string)

where flannel-opts include flannel-ipv6-masq and flannel-external-ip (e.g. `--flannel-opts=flannel-ipv6-masq`, in this case flannel-ipv6-masq=true flannel-external-ip=false)

We could also reduce it to 1:

* flannel-opts ([]string)

where flannel-opts include flannel-backend, flannel-ipv6-masq and flannel-external-ip (e.g. `--flannel-opts=backend=wireguard-native,flannel-ipv6-masq`, in this case flannel-backend=wireguard-native flannel-ipv6-masq=true flannel-external-ip=false)


## Decision



## Consequences
