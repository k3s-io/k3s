[![Linux Build Status](https://travis-ci.org/containernetworking/plugins.svg?branch=master)](https://travis-ci.org/containernetworking/plugins)
[![Windows Build Status](https://ci.appveyor.com/api/projects/status/kcuubx0chr76ev86/branch/master?svg=true)](https://ci.appveyor.com/project/cni-bot/plugins/branch/master)

# plugins
Some CNI network plugins, maintained by the containernetworking team. For more information, see the individual READMEs.

## Plugins supplied:
### Main: interface-creating
* `bridge`: Creates a bridge, adds the host and the container to it.
* `ipvlan`: Adds an [ipvlan](https://www.kernel.org/doc/Documentation/networking/ipvlan.txt) interface in the container
* `loopback`: Creates a loopback interface
* `macvlan`: Creates a new MAC address, forwards all traffic to that to the container
* `ptp`: Creates a veth pair.
* `vlan`: Allocates a vlan device.

### IPAM: IP address allocation
* `dhcp`: Runs a daemon on the host to make DHCP requests on behalf of the container
* `host-local`: maintains a local database of allocated IPs

### Meta: other plugins
* `flannel`: generates an interface corresponding to a flannel config file
* `tuning`: Tweaks sysctl parameters of an existing interface
* `portmap`: An iptables-based portmapping plugin. Maps ports from the host's address space to the container.

### Sample
The sample plugin provides an example for building your own plugin.
