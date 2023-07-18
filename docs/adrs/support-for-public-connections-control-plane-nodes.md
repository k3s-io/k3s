# Support for public connection between servers

Date: 2023-07-18

## Status

Proposed

## Context

A user raised an [issue](https://github.com/k3s-io/k3s/issues/2965) requesting that the embedded etcd of each server should be able to communicate with other servers in clusters outside the LAN connection.

The use case proposed in the issue involves scenarios where servers are not in the same LAN connection but have LAN-like latency between them.

Currently, k3s allows for the deployment of a server cluster, but only within the same LAN connection or with a VPN. For example, when a user creates a server cluster, they pass the `advertise-address` flag in the CLI. However, the embedded etcd does not utilize this `advertise-address` flag and instead always uses the private IP for connections between etcd nodes.

My proposal is to introduce an additional flag in the CLI that indicates that etcd will now use the `advertise-address` flag. With this, we can send a new item in the etcd configuration, allowing a server attempting to join the cluster to receive the member list with the public IP of the cluster.

### Architecture

Digging deeper into the code, here is a high-level summary of the changes that will be applied to k3s and etcd:
* [k3s] A new flag will be added for the server, which provides a boolean value to determine whether etcd will use the `advertise-address` to advertise to other etcds in the cluster.
* [etcd] etcd will now accept different ips for the cluster value. 
	
For example:
* Server 1 value will be initialCluster = Server1Name=Server1PrivateIp,Server2Name=Server2PublicIp
* Server 2 value will be initialCluster = Server1Name=Server1PublicIp,Server2Name=Server2PrivateIp

## Decision

[No decision has been made yet.]

## Consequences

Good
====
* Users can deploy k3s control plane clusters in seconds that seamlessly work and connect heterogeneous nodes
* Fills the gap for use cases that need clusters without LAN connection and without VPN

Bad
===
* May cause problems if the connection between the etcd cluster are poor.

