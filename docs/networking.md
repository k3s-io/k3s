
# Networking

This page explains how CoreDNS, the Traefik Ingress controller, and Klipper service load balancer work within K3s.

Refer to the [Installation Network Options](install/network_options.md) page for details on Flannel configuration options and backend selection, or how to set up your own CNI.

For information on which ports need to be opened for K3s, refer to the [Installation Requirements.](install/install-requirements/install_requirements.md#networking)

- [CoreDNS](#coredns)
- [Traefik Ingress Controller](#traefik-ingress-controller)
- [Service Load Balancer](#service-load-balancer)
  - [How the Service LB Works](#how-the-service-lb-works)
  - [Usage](#usage)
  - [Excluding the Service LB from Nodes](#excluding-the-service-lb-from-nodes)
  - [Disabling the Service LB](#disabling-the-service-lb)
- [Nodes Without a Hostname](#nodes-without-a-hostname)

## CoreDNS

CoreDNS is deployed on start of the agent. To disable, run each server with the `--disable coredns` option.

If you don't install CoreDNS, you will need to install a cluster DNS provider yourself.

## Traefik Ingress Controller

[Traefik](https://traefik.io/) is a modern HTTP reverse proxy and load balancer made to deploy microservices with ease. It simplifies networking complexity while designing, deploying, and running applications.

Traefik is deployed by default when starting the server. For more information see [Auto Deploying Manifests](advanced.md#auto-deploying-manifests). The default config file is found in `/var/lib/rancher/k3s/server/manifests/traefik.yaml`.

The Traefik ingress controller will use ports 80 and 443 on the host (i.e. these will not be usable for HostPort or NodePort).

The `traefik.yaml` file should not be edited manually, because k3s would overwrite it again once it is restarted. Instead you can customize Traefik by creating an additional `HelmChartConfig` manifest in `/var/lib/rancher/k3s/server/manifests`. For more details and an example see [Customizing Packaged Components with HelmChartConfig](helm.md#customizing-packaged-components-with-helmchartconfig). For more information on the possible configuration values, refer to the official [Traefik Helm Configuration Parameters.](https://github.com/traefik/traefik-helm-chart/tree/master/traefik).

To disable it, start each server with the `--disable traefik` option.

If Traefik is not disabled K3s versions 1.20 and earlier will install Traefik v1, while K3s versions 1.21 and later will install Traefik v2 if v1 is not already present.

To migrate from an older Traefik v1 instance please refer to the [Traefik documentation](https://doc.traefik.io/traefik/migration/v1-to-v2/) and [migration tool](https://github.com/traefik/traefik-migration-tool).

## Service Load Balancer

Any service load balancer (LB) can be leveraged in your Kubernetes cluster. K3s provides a load balancer known as [Klipper Load Balancer](https://github.com/k3s-io/klipper-lb) that uses available host ports.

Upstream Kubernetes allows a Service of type LoadBalancer to be created, but doesn't include the implementation of the LB. Some LB services require a cloud provider such as Amazon EC2 or Microsoft Azure. By contrast, the K3s service LB makes it possible to use an LB service without a cloud provider.

### How the Service LB Works

K3s creates a controller that creates a Pod for the service load balancer, which is a Kubernetes object of kind [Service.](https://kubernetes.io/docs/concepts/services-networking/service/)

For each service load balancer, a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) is created. The DaemonSet creates a pod with the `svc` prefix on each node.

The Service LB controller listens for other Kubernetes Services. After it finds a Service, it creates a proxy Pod for the service using a DaemonSet on all of the nodes. This Pod becomes a proxy to the other Service, so that for example, requests coming to port 8000 on a node could be routed to your workload on port 8888.

If the Service LB runs on a node that has an external IP, it uses the external IP.

If multiple Services are created, a separate DaemonSet is created for each Service.

It is possible to run multiple Services on the same node, as long as they use different ports.

If you try to create a Service LB that listens on port 80, the Service LB will try to find a free host in the cluster for port 80. If no host with that port is available, the LB will stay in Pending.

### Usage

Create a [Service of type LoadBalancer](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) in K3s.

### Excluding the Service LB from Nodes

To exclude nodes from using the Service LB, add the following label to the nodes that should not be excluded:

```
svccontroller.k3s.cattle.io/enablelb
```

If the label is used, the service load balancer only runs on the labeled nodes.

### Disabling the Service LB

To disable the embedded LB, run the server with the `--disable servicelb` option.

This is necessary if you wish to run a different LB, such as MetalLB.

## Nodes Without a Hostname

Some cloud providers, such as Linode, will create machines with "localhost" as the hostname and others may not have a hostname set at all. This can cause problems with domain name resolution. You can run K3s with the `--node-name` flag or `K3S_NODE_NAME` environment variable and this will pass the node name to resolve this issue.
