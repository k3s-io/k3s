# Package spegel Distributed Registry Mirror

Date: 2023-12-07

## Status

Accepted

## Context

Embedded registry mirror support has been on the roadmap for some time, to address multiple challenges:
* Upstream registries may enforce pull limits or otherwise throttle access to images.
* In edge scenarios, bandwidth is at a premium, if external access is available at all.
* Distributing airgap image tarballs to nodes, and ensuring that images remain available, is an ongoing
  hurdle to adoption.
* Deploying an in-cluster registry, or hosting a registry outside the cluster, put significant
  burden on administrators, and suffer from chicken-or-egg bootstrapping issues.

An ideal embedded registry would have several characteristics:
* Allow stateless configuration such that nodes can come and go at any time.
* Integrate into existing containerd registry mirror support.
* Integrate into existing containerd image stores such that an additional copy of layer data is not required.
* Use existing cluster authentication mechanisms to prevent unauthorized access to the registry.
* Operate with minimal added CPU and memory overhead.

## Decision

* We will embed spegel within K3s, and use it to host a distributed registry mirror.
* The distributed registry mirror will be enabled cluster-wide via server CLI flag.
* Selection of upstream registries to mirror will be implemented via the existing `registries.yaml`
configuration file.
* The registry API will be served via HTTPS on every node's private IP at port 6443. On servers this will
use the existing supervisor listener; on agents a new listener will be created for this purpose.
* The default IPFS/libp2p port of 5001 will be used for P2P layer discovery.
* Access to the registry API and P2P network will require proof of cluster membership, enforced via
client certificate or preshared key.
* Hybrid/multicloud support is out of scope; when the distributed registry mirror is enabled, cluster
members are assumed to be directly accessible to each other via their internal IP on the listed ports.

## Consequences

* The size of our self-extracting binary and Docker images increase by several megabytes.
* We take on the support burden of keeping spegel up to date, and supporting its use within K3s.
