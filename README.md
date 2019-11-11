k3s - 5 less than k8s
===============================================

Lightweight Kubernetes.  Easy to install, half the memory, all in a binary less than 40mb.

Great for:

* Edge
* IoT
* CI
* ARM
* Situations where a PhD in k8s clusterology is infeasible

What is this?
---

k3s is intended to be a fully compliant Kubernetes distribution with the following changes:

1. Legacy, alpha, non-default features are removed. Hopefully, you shouldn't notice the
   stuff that has been removed.
2. Removed most in-tree plugins (cloud providers and storage plugins) which can be replaced
   with out of tree addons.
3. Add sqlite3 as the default storage mechanism. etcd3 is still available, but not the default.
4. Wrapped in simple launcher that handles a lot of the complexity of TLS and options.
5. Minimal to no OS dependencies (just a sane kernel and cgroup mounts needed). k3s packages required
   dependencies
    * containerd
    * Flannel
    * CoreDNS
    * CNI
    * Host utilities (iptables, socat, etc)


Documentation
-------------

Please see [the official docs site](https://rancher.com/docs/k3s/latest/en/) for complete documentation on k3s.

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

Building from Source
--------------------

See the [release](https://github.com/rancher/k3s/releases/latest) page for pre-built releases.

The clone will be much faster on this repo if you do

    git clone --depth 1 https://github.com/rancher/k3s.git

This repo includes all of Kubernetes history so `--depth 1` will avoid most of that.

To build the full release binary run `make` and that will create `./dist/artifacts/k3s`.

Optionally to build the binaries without running linting or building docker images:
```sh
./scripts/download && ./scripts/build && ./scripts/package-cli
```

For development, you just need go 1.12 and a sane GOPATH.  To compile the binaries run:
```bash
go build -o k3s
go build -o kubectl ./cmd/kubectl
go build -o hyperkube ./vendor/k8s.io/kubernetes/cmd/hyperkube
```

This will create the main executable, but it does not include the dependencies like containerd, CNI,
etc.  To run a server and agent with all the dependencies for development run the following
helper scripts:
```bash
# Server
./scripts/dev-server.sh

# Agent
./scripts/dev-agent.sh
```

Kubernetes Source
-----------------

The source code for Kubernetes is in `vendor/` and the location from which that is copied
is in `./vendor.conf`.  Go to the referenced repo/tag and you'll find all the patches applied
to upstream Kubernetes.
