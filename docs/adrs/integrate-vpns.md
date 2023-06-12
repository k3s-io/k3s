# Integrate vpn in k3s

Date: 2023-04-26

## Status

Under review

## Context

There are kubernetes use cases which require a kubernetes cluster to be deployed on a set of heterogeneous nodes, i.e. baremetal nodes, AWS VMs, Azure VMs, etc. Some of these use cases:
	* Edge apps which are divided into two parts: a small footprint part deployed at the edge and the "non-edge" part deployed in the DC. These need to be always connected.
	* Having a baremetal cluster that requires, only in certain periods, to be extended with hyperscalers VMs to cope out with the demand
	* Require cluster to include nodes in different hyperscalers due to resiliency reasons or legal requirements, e.g. GDPR

As of today, k3s allows to deploy a cluster on a set of heterogeneous nodes by a simple and robust solution. This is achieved by using the [websocket proxy](https://github.com/k3s-io/k3s/blob/master/pkg/agent/run.go#L277) to connect the control-plane of the cluster, i.e. kube-api <==> kubelet, and a vpn-type flannel backend, e.g. wireguard, to connect the data-plane, i.e. pod <==> pod/node.

The current solution works well but has a few limitations:
	* It requires the server to have a public IP
	* It requires the server to open ports on that external IP (e.g. 6443)
    * Projects like prometheus or metrics-server that attempt to scrape nodes directly will not work, as pod --> host traffic does not work from scratch
	* There is no central management point for your vpn. Therefore, it is impossible to:
		1. Have a vpn topology view 
		2. Monitor node status, performance, etc
		3. Configure ACLs or other policies
		4. Other features

There are well known projects which can be used as an alternative to our solution. In general, these projects set up a vpn mesh that includes all nodes and thus we could deploy k3s as if all nodes belonged to the same network. Besides, these projects include a central management point that offer extra features and do not require a public IP to be available. Some of these projects are: tailscale, netmaker, nebula or zerotier.

We already have users that are operating k3s on top of one of these vpn solutions. However, it is sometimes a pain for them because they are not necessarily network experts and run into integration problems such as: performance issues due to double encapsulation, [mtu issues due to vpn tunneling encapsulation](https://github.com/k3s-io/k3s/issues/4743), strange errors due to wrong vpn configuration, etc. Moreover, they need to first deploy the vpn, collect important information and then deploy k3s using that information. These three steps are not always easy to automate and automation is paramount in edge use cases.

My proposal is to integrate the best or the two best of these projects into k3s. Integrating in the sense of setting up the vpn and configuring k3s accordingly, so that the user ends up with a working heterogeneous cluster in a matter of seconds by just deploying k3s. At this stage, the proposal is not to incorporate the vpn binaries or daemons inside the k3s binary, we require the user to have the vpn binary installed.

Therefore, the user would have 1 or 2 alternatives to deploy k3s in an heterogeneous cluster:
1 - Our simple and robust solution
2 - vpn solution (e.g. tailscale)
3 - (Optional) vpn solution 2 (e.g. netmaker)

In terms of support, we could always offer support for alternative 1 and best effort support for alternative 2 (or 3). We don't control those projects and some of them have proprietary parts or are using a freemium business model (e.g. lmimited number nodes)

In the first round, only tailscale will be integrated

### Architecture

Going a bit deeper into the code, this is a high-level summary of the changes applied to k3s:
	* New flag is passed for both server and agent. This flag provide the name and the auth-keys required for the node to be accepted in the vpn and set node configs (e.g. allow routing of podCIDR via the VPN)
	* Functions that start the vpn and provide information of its status in "package netutil" (pkg/netutil/vpn.go)
	* VPNInfo struct in "package netutil" that includes important vpn endpoint information such as the vpn endpoint IP and the nodeID in the vpn context (for config purposes)
	* The collection of vpn endpoint information and its start are implemented by calling the vpn binary. Tailscale has a "not-feature-complete" go client but netmaker does not, so calls to the binary is the common denominator
	* In the agents, if a vpn config flag is detected, the vpn is started before the websocket proxy is created, so the agent can contact the server
	* In the servers, if a vpn config flag is detected, the vpn is started before the apiserver is started, so that agents can contact the server. AdvertiseIP is changed to use the VPN IP
	* If a vpn config flag is detected, the vpn info is collected and the nodeIP replaced before the kubelet certificate is created (due to SAN). This happens in func get(...) of pkg/agent/config/config.go
	* A flannel backend is defined: tailscale. These use the general purpose "extension" backend, which executes shell commands when certain events happen (e.g. new node is added)
	* When a new node is added, flannel queries the subnet podCIDR for that node. The new backends, by executing the vpn binary with certain flags, allow traffic to/from that subnet podCIDR to flow via the VPN
    * In HA scenarios, etcd IP will not use the VPN IP (serverConfig.ControlConfig.PrivateIP) but the main interface. Running etcd traffic over the internet does not make sense. Therefore, k3s-HA over VPN is not supported


## Decision

???

## Consequences

Good
====
* Users can automatically deploy vpn+k3s in seconds that seamlessly work and connect heterogeneous nodes
* New exciting feature for the community
* We offer not only our simple solution but some extra ones for heterogeneous clusters
* Fills the gap for useful use cases in edge

Bad
===
* Integration with 3rd party projects which we do not control and thus complete support is not possible (similar to CNI plugins)
* Some of these projects are not 100% open source (e.g. tailscale) and some are in its infancy (i.e. buggy), e.g. netmaker.
* Not possible to configure a set of heterogeneous nodes in Rancher Management. Therefore, it is currently impossible to deploy through it but could be deployed standalone

