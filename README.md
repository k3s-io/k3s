k3s - Lightweight Kubernetes
===============================================

Lightweight Kubernetes.  Easy to install, half the memory, all in a binary less than 100 MB.

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

k3s is a fully compliant Kubernetes distribution with the following changes:

1. Packaged as a single binary.
1. Lightweight storage backend based on sqlite3 as the default storage mechanism. etcd3, MySQL, Postgres also still available.
1. Wrapped in simple launcher that handles a lot of the complexity of TLS and options.
1. Secure by default with reasonable defaults for lightweight environments.
1. Minimal to no OS dependencies (just a sane kernel and cgroup mounts needed). k3s packages required
   dependencies
    * containerd
    * Flannel
    * CoreDNS
    * CNI
    * Host utilities (iptables, socat, etc)
    * Ingress controller (traefik)
    * Embedded service loadbalancer
    * Embedded network policy controller

What's with the name?
--------------------
We wanted an installation of Kubernetes that was half the size in terms of memory footprint. Kubernetes is a
10 letter word stylized as k8s. So something half as big as Kubernetes would be a 5 letter word stylized as
k3s. There is no long form of k3s and no official pronunciation.

Documentation
-------------

Please see [the official docs site](https://rancher.com/docs/k3s/latest/en/) for complete documentation on k3s.

## Contributing to the Docs

- **Issues:** Doc issues are raised in this repository, and they are tracked under the `kind/documentation` label. 
- **Pull Requests:** Pull requests are submitted to the K3s documentation source code in the [Rancher docs repository.](https://github.com/rancher/docs/) The K3s docs content is in the `content/k3s/` directory.

Quick-Start - Install Script
--------------

The k3s `install.sh` script provides a convenient way for installing to systemd or openrc,
to install k3s as a service just run:

```bash
curl -sfL https://get.k3s.io | sh -
```

A kubeconfig file is written to `/etc/rancher/k3s/k3s.yaml` and the service is automatically started or restarted.
The install script will install k3s and additional utilities, such as `kubectl`, `crictl`, `k3s-killall.sh`, and `k3s-uninstall.sh`, for example:

```bash
sudo kubectl get nodes
```

`K3S_TOKEN` is created at `/var/lib/rancher/k3s/server/node-token` on your server.
To install on worker nodes we should pass `K3S_URL` along with
`K3S_TOKEN` or `K3S_CLUSTER_SECRET` environment variables, for example:

```bash
curl -sfL https://get.k3s.io | K3S_URL=https://myserver:6443 K3S_TOKEN=XXX sh -
```

Manual Download
---------------

1. Download `k3s` from latest [release](https://github.com/rancher/k3s/releases/latest), x86_64, armhf, and arm64 are supported.
2. Run server.

```bash
sudo k3s server &
# Kubeconfig is written to /etc/rancher/k3s/k3s.yaml
sudo k3s kubectl get nodes

# On a different node run the below. NODE_TOKEN comes from
# /var/lib/rancher/k3s/server/node-token on your server
sudo k3s agent --server https://myserver:6443 --token ${NODE_TOKEN}
```

