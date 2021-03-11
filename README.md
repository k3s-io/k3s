K3s - Lightweight Kubernetes
===============================================

Lightweight Kubernetes.  Production ready, easy to install, half the memory, all in a binary less than 100 MB.

Great for:

* Edge
* IoT
* CI
* Development
* ARM
* Embedding k8s
* Situations where a PhD in k8s clusterology is infeasible

What is this?
---

K3s is a [fully conformant](https://github.com/cncf/k8s-conformance/pulls?q=is%3Apr+k3s) production-ready Kubernetes distribution with the following changes:

1. It is packaged as a single binary.
1. It adds support for sqlite3 as the default storage backend. Etcd3, MySQL, and Postgres are also supported.
1. It wraps Kubernetes and other components in a single, simple launcher.
1. It is secure by default with reasonable defaults for lightweight environments.
1. It has minimal to no OS dependencies (just a sane kernel and cgroup mounts needed).
1. It eliminates the need to expose a port on Kubernetes worker nodes for the kubelet API by exposing this API to the Kubernetes control plane nodes over a websocket tunnel.

K3s bundles the following technologies together into a single cohesive distribution:

* [Containerd](https://containerd.io/) & [runc](https://github.com/opencontainers/runc)
* [Flannel](https://github.com/coreos/flannel) for CNI
* [CoreDNS](https://coredns.io/)
* [Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
* [Traefik](https://containo.us/traefik/) for ingress
* [Klipper-lb](https://github.com/k3s-io/klipper-lb) as an embedded service load balancer provider
* [Kube-router](https://www.kube-router.io/) for network policy
* [Helm-controller](https://github.com/k3s-io/helm-controller) to allow for CRD-driven deployment of helm manifests
* [Kine](https://github.com/k3s-io/kine) as a datastore shim that allows etcd to be replaced with other databases
* [Local-path-provisioner](https://github.com/rancher/local-path-provisioner) for provisioning volumes using local storage
* [Host utilities](https://github.com/k3s-io/k3s-root) such as iptables/nftables, ebtables, ethtool, & socat

These technologies can be disabled or swapped out for technologies of your choice.

Additionally, K3s simplifies Kubernetes operations by maintaining functionality for:

* Managing the TLS certificates of Kubernetes components
* Managing the connection between worker and server nodes
* Auto-deploying Kubernetes resources from local manifests in realtime as they are changed.
* Managing an embedded etcd cluster (work in progress)

What's with the name?
--------------------

We wanted an installation of Kubernetes that was half the size in terms of memory footprint. Kubernetes is a
10 letter word stylized as k8s. So something half as big as Kubernetes would be a 5 letter word stylized as
K3s. There is neither a long-form of K3s nor official pronunciation.

Is this a fork?
---------------

No, it's a distribution. A fork implies continued divergence from the original. This is not K3s's goal or practice. K3s explicitly intends not to change any core Kubernetes functionality. We seek to remain as close to upstream Kubernetes as possible. However, we maintain a small set of patches (well under 1000 lines) important to K3s's use case and deployment model. We maintain patches for other components as well. When possible, we contribute these changes back to the upstream projects, for example, with [SELinux support in containerd](https://github.com/containerd/cri/pull/1487/commits/24209b91bf361e131478d15cfea1ab05694dc3eb). This is a common practice amongst software distributions.

K3s is a distribution because it packages additional components and services necessary for a fully functional cluster that go beyond vanilla Kubernetes. These are opinionated choices on technologies for components like ingress, storage class, network policy, service load balancer, and even container runtime. These choices and technologies are touched on in more detail in the [What is this?](#what-is-this) section.

How is this lightweight or smaller than upstream Kubernetes?
---

There are two major ways that K3s is lighter weight than upstream Kubernetes:
1. The memory footprint to run it is smaller
1. The binary, which contains all the non-containerized components needed to run a cluster, is smaller

The memory footprint is reduced primarily by running many components inside of a single process. This eliminates significant overhead that would otherwise be duplicated for each component.

The binary is smaller by removing third-party storage drivers and cloud providers, explained in more detail below.

What have you removed from upstream Kubernetes?
---

This is a common point of confusion because it has changed over time. Early versions of K3s had much more removed than the current version. K3s currently removes two things:

1. In-tree storage drivers
1. In-tree cloud provider

Both of these have out-of-tree alternatives in the form of [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [CCM](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/), which work in K3s and which upstream is moving towards.

We remove these to achieve a smaller binary size. They can be removed while remaining conformant because neither affects core Kubernetes functionality. They are also dependent on third-party cloud or data center technologies/services, which may not be available in many K3s' use cases.

What's next?
---

Check out our [roadmap](ROADMAP.md) to see what we have planned moving forward.

Release cadence
---

K3s maintains pace with upstream Kubernetes releases. Our goal is to release patch releases on the same day as upstream and minor releases within a few days.

Our release versioning reflects the version of upstream Kubernetes that is being released. For example, the K3s release [v1.18.6+k3s1](https://github.com/k3s-io/k3s/releases/tag/v1.18.6%2Bk3s1) maps to the `v1.18.6` Kubernetes release. We add a postfix in the form of `+k3s<number>` to allow us to make additional releases using the same version of upstream Kubernetes while remaining [semver](https://semver.org/) compliant. For example, if we discovered a high severity bug in `v1.18.6+k3s1` and needed to release an immediate fix for it, we would release `v1.18.6+k3s2`.

Documentation
-------------

Please see [the official docs site](https://rancher.com/docs/k3s/latest/en/) for complete documentation.

Quick-Start - Install Script
--------------

The `install.sh` script provides a convenient way to download K3s and add a service to systemd or openrc.

To install k3s as a service, run:

```bash
curl -sfL https://get.k3s.io | sh -
```

A kubeconfig file is written to `/etc/rancher/k3s/k3s.yaml` and the service is automatically started or restarted.
The install script will install K3s and additional utilities, such as `kubectl`, `crictl`, `k3s-killall.sh`, and `k3s-uninstall.sh`, for example:

```bash
sudo kubectl get nodes
```

`K3S_TOKEN` is created at `/var/lib/rancher/k3s/server/node-token` on your server.
To install on worker nodes, pass `K3S_URL` along with
`K3S_TOKEN` or `K3S_CLUSTER_SECRET` environment variables, for example:

```bash
curl -sfL https://get.k3s.io | K3S_URL=https://myserver:6443 K3S_TOKEN=XXX sh -
```

Manual Download
---------------

1. Download `k3s` from latest [release](https://github.com/k3s-io/k3s/releases/latest), x86_64, armhf, and arm64 are supported.
1. Run the server.

```bash
sudo k3s server &
# Kubeconfig is written to /etc/rancher/k3s/k3s.yaml
sudo k3s kubectl get nodes

# On a different node run the below. NODE_TOKEN comes from
# /var/lib/rancher/k3s/server/node-token on your server
sudo k3s agent --server https://myserver:6443 --token ${NODE_TOKEN}
```

Contributing
------------

Please check out our [contributing guide](CONTRIBUTING.md) if you're interested in contributing to K3s.

Security
--------

Security issues in K3s can be reported by sending an email to [security@k3s.io](mailto:security@k3s.io). Please do not file issues about security issues.
