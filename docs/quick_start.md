---
title: "Quick-Start Guide"
weight: 10
---

This guide will help you quickly launch a cluster with default options. The [installation section](../installation) covers in greater detail how K3s can be set up.

For information on how K3s components work together, refer to the [architecture section.]({{<baseurl>}}/k3s/latest/en/architecture/#high-availability-with-an-external-db)

> New to Kubernetes? The official Kubernetes docs already have some great tutorials outlining the basics [here](https://kubernetes.io/docs/tutorials/kubernetes-basics/).

Install Script
--------------
K3s provides an installation script that is a convenient way to install it as a service on systemd or openrc based systems. This script is available at https://get.k3s.io. To install K3s using this method, just run:
```bash
curl -sfL https://get.k3s.io | sh -
```

After running this installation:

* The K3s service will be configured to automatically restart after node reboots or if the process crashes or is killed
* Additional utilities will be installed, including `kubectl`, `crictl`, `ctr`, `k3s-killall.sh`, and `k3s-uninstall.sh`
* A [kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file will be written to `/etc/rancher/k3s/k3s.yaml` and the kubectl installed by K3s will automatically use it

To install on worker nodes and add them to the cluster, run the installation script with the `K3S_URL` and `K3S_TOKEN` environment variables. Here is an example showing how to join a worker node:

```bash
curl -sfL https://get.k3s.io | K3S_URL=https://myserver:6443 K3S_TOKEN=mynodetoken sh -
```
Setting the `K3S_URL` parameter causes K3s to run in worker mode. The K3s agent will register with the K3s server listening at the supplied URL. The value to use for `K3S_TOKEN` is stored at `/var/lib/rancher/k3s/server/node-token` on your server node.

Note: Each machine must have a unique hostname. If your machines do not have unique hostnames, pass the `K3S_NODE_NAME` environment variable and provide a value with a valid and unique hostname for each node.
