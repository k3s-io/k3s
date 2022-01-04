
# Advanced Options and Configuration

This section contains advanced information describing the different ways you can run and manage K3s:

- [Certificate rotation](#certificate-rotation)
- [Auto-deploying manifests](#auto-deploying-manifests)
- [Using Docker as the container runtime](#using-docker-as-the-container-runtime)
- [Using etcdctl](#using-etcdctl)
- [Configuring containerd](#configuring-containerd)
- [Running K3s with Rootless mode (Experimental)](#running-k3s-with-rootless-mode-experimental)
- [Node labels and taints](#node-labels-and-taints)
- [Starting the server with the installation script](#starting-the-server-with-the-installation-script)
- [Additional preparation for Alpine Linux setup](#additional-preparation-for-alpine-linux-setup)
- [Running K3d (K3s in Docker) and docker-compose](#running-k3d-k3s-in-docker-and-docker-compose)
- [Enabling legacy iptables on Raspbian Buster](#enabling-legacy-iptables-on-raspbian-buster)
- [Enabling cgroups for Raspbian Buster](#enabling-cgroups-for-raspbian-buster)
- [SELinux Support](#selinux-support)
- [Additional preparation for (Red Hat/CentOS) Enterprise Linux](#additional-preparation-for-red-hat-centos-enterprise-linux)
- [Enabling Lazy Pulling of eStargz (Experimental)](#enabling-lazy-pulling-of-estargz-experimental)
- [Additional Logging Sources](#additional-logging-sources)
- [Server and agent tokens](#server-and-agent-tokens)

## Certificate Rotation

By default, certificates in K3s expire in 12 months.

If the certificates are expired or have fewer than 90 days remaining before they expire, the certificates are rotated when K3s is restarted.

## Auto-Deploying Manifests

Any file found in `/var/lib/rancher/k3s/server/manifests` will automatically be deployed to Kubernetes in a manner similar to `kubectl apply`, both on startup and when the file is changed on disk. Deleting files out of this directory will not delete the corresponding resources from the cluster.

For information about deploying Helm charts, refer to the section about [Helm.](../helm)

## Using Docker as the Container Runtime

K3s includes and defaults to [containerd,](https://containerd.io/) an industry-standard container runtime.

To use Docker instead of containerd,

1. Install Docker on the K3s node. One of Rancher's [Docker installation scripts](https://github.com/rancher/install-docker) can be used to install Docker:

    ```
    curl https://releases.rancher.com/install-docker/19.03.sh | sh
    ```

1. Install K3s using the `--docker` option:

    ```
    curl -sfL https://get.k3s.io | sh -s - --docker
    ```

1. Confirm that the cluster is available:

    ```
    $ sudo k3s kubectl get pods --all-namespaces
    NAMESPACE     NAME                                     READY   STATUS      RESTARTS   AGE
    kube-system   local-path-provisioner-6d59f47c7-lncxn   1/1     Running     0          51s
    kube-system   metrics-server-7566d596c8-9tnck          1/1     Running     0          51s
    kube-system   helm-install-traefik-mbkn9               0/1     Completed   1          51s
    kube-system   coredns-8655855d6-rtbnb                  1/1     Running     0          51s
    kube-system   svclb-traefik-jbmvl                      2/2     Running     0          43s
    kube-system   traefik-758cd5fc85-2wz97                 1/1     Running     0          43s
    ```

1. Confirm that the Docker containers are running:

    ```
    $ sudo docker ps
    CONTAINER ID        IMAGE                     COMMAND                  CREATED              STATUS              PORTS               NAMES
    3e4d34729602        897ce3c5fc8f              "entry"                  About a minute ago   Up About a minute                       k8s_lb-port-443_svclb-traefik-jbmvl_kube-system_d46f10c6-073f-4c7e-8d7a-8e7ac18f9cb0_0
    bffdc9d7a65f        rancher/klipper-lb        "entry"                  About a minute ago   Up About a minute                       k8s_lb-port-80_svclb-traefik-jbmvl_kube-system_d46f10c6-073f-4c7e-8d7a-8e7ac18f9cb0_0
    436b85c5e38d        rancher/library-traefik   "/traefik --configfi…"   About a minute ago   Up About a minute                       k8s_traefik_traefik-758cd5fc85-2wz97_kube-system_07abe831-ffd6-4206-bfa1-7c9ca4fb39e7_0
    de8fded06188        rancher/pause:3.1         "/pause"                 About a minute ago   Up About a minute                       k8s_POD_svclb-traefik-jbmvl_kube-system_d46f10c6-073f-4c7e-8d7a-8e7ac18f9cb0_0
    7c6a30aeeb2f        rancher/pause:3.1         "/pause"                 About a minute ago   Up About a minute                       k8s_POD_traefik-758cd5fc85-2wz97_kube-system_07abe831-ffd6-4206-bfa1-7c9ca4fb39e7_0
    ae6c58cab4a7        9d12f9848b99              "local-path-provisio…"   About a minute ago   Up About a minute                       k8s_local-path-provisioner_local-path-provisioner-6d59f47c7-lncxn_kube-system_2dbd22bf-6ad9-4bea-a73d-620c90a6c1c1_0
    be1450e1a11e        9dd718864ce6              "/metrics-server"        About a minute ago   Up About a minute                       k8s_metrics-server_metrics-server-7566d596c8-9tnck_kube-system_031e74b5-e9ef-47ef-a88d-fbf3f726cbc6_0
    4454d14e4d3f        c4d3d16fe508              "/coredns -conf /etc…"   About a minute ago   Up About a minute                       k8s_coredns_coredns-8655855d6-rtbnb_kube-system_d05725df-4fb1-410a-8e82-2b1c8278a6a1_0
    c3675b87f96c        rancher/pause:3.1         "/pause"                 About a minute ago   Up About a minute                       k8s_POD_coredns-8655855d6-rtbnb_kube-system_d05725df-4fb1-410a-8e82-2b1c8278a6a1_0
    4b1fddbe6ca6        rancher/pause:3.1         "/pause"                 About a minute ago   Up About a minute                       k8s_POD_local-path-provisioner-6d59f47c7-lncxn_kube-system_2dbd22bf-6ad9-4bea-a73d-620c90a6c1c1_0
    64d3517d4a95        rancher/pause:3.1         "/pause"
    ```

### Optional: Use crictl with Docker

crictl provides a CLI for CRI-compatible container runtimes.

If you would like to use crictl after installing K3s with the `--docker` option, install crictl using the [official documentation:](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/crictl.md) 

```
$ VERSION="v1.17.0"
$ curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-${VERSION}-linux-amd64.tar.gz --output crictl-${VERSION}-linux-amd64.tar.gz
$ sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
crictl
```

Then start using crictl commands:

```
$ sudo crictl version
Version:  0.1.0
RuntimeName:  docker
RuntimeVersion:  19.03.9
RuntimeApiVersion:  1.40.0
$ sudo crictl images
IMAGE                            TAG                 IMAGE ID            SIZE
rancher/coredns-coredns          1.6.3               c4d3d16fe508b       44.3MB
rancher/klipper-helm             v0.2.5              6207e2a3f5225       136MB
rancher/klipper-lb               v0.1.2              897ce3c5fc8ff       6.1MB
rancher/library-traefik          1.7.19              aa764f7db3051       85.7MB
rancher/local-path-provisioner   v0.0.11             9d12f9848b99f       36.2MB
rancher/metrics-server           v0.3.6              9dd718864ce61       39.9MB
rancher/pause                    3.1                 da86e6ba6ca19       742kB
```

## Using etcdctl

etcdctl provides a CLI for etcd.

If you would like to use etcdctl after installing K3s with embedded etcd, install etcdctl using the [official documentation.](https://etcd.io/docs/latest/install/) 

```
$ VERSION="v3.5.0"
$ curl -L https://github.com/etcd-io/etcd/releases/download/${VERSION}/etcd-${VERSION}-linux-amd64.tar.gz --output etcdctl-linux-amd64.tar.gz
$ sudo tar -zxvf etcdctl-linux-amd64.tar.gz --strip-components=1 -C /usr/local/bin etcd-${VERSION}-linux-amd64/etcdctl
```

Then start using etcdctl commands with the appropriate K3s flags:

```
$ sudo etcdctl --cacert=/var/lib/rancher/k3s/server/tls/etcd/server-ca.crt --cert=/var/lib/rancher/k3s/server/tls/etcd/client.crt --key=/var/lib/rancher/k3s/server/tls/etcd/client.key version
```

## Configuring containerd

K3s will generate config.toml for containerd in `/var/lib/rancher/k3s/agent/etc/containerd/config.toml`.

For advanced customization for this file you can create another file called `config.toml.tmpl` in the same directory and it will be used instead.

The `config.toml.tmpl` will be treated as a Go template file, and the `config.Node` structure is being passed to the template. [This template](https://github.com/rancher/k3s/blob/master/pkg/agent/templates/templates.go#L16-L32) example on how to use the structure to customize the configuration file.


## Running K3s with Rootless mode (Experimental)

> **Warning:** This feature is experimental.

Rootless mode allows running the entire k3s an unprivileged user, so as to protect the real root on the host from potential container-breakout attacks.

See also https://rootlesscontaine.rs/ to learn about Rootless mode.

### Known Issues with Rootless mode

* **Ports**

    When running rootless a new network namespace is created.  This means that K3s instance is running with networking fairly detached from the host.  The only way to access services run in K3s from the host is to set up port forwards to the K3s network namespace. We have a controller that will automatically bind 6443 and service port below 1024 to the host with an offset of 10000. 

    That means service port 80 will become 10080 on the host, but 8080 will become 8080 without any offset.

    Currently, only `LoadBalancer` services are automatically bound.

* **Cgroups**

    Cgroup v1 is not supported. v2 is supported.

* **Multi-node cluster**

    Multi-cluster installation is untested and undocumented.

### Running Servers and Agents with Rootless
* Enable cgroup v2 delegation, see https://rootlesscontaine.rs/getting-started/common/cgroup2/ .
  This step is optional, but highly recommended for enabling CPU and memory resource limtitation.

* Download `k3s-rootless.service` from [`https://github.com/k3s-io/k3s/blob/<VERSION>/k3s-rootless.service`](https://github.com/k3s-io/k3s/blob/master/k3s-rootless.service).
  Make sure to use the same version of `k3s-rootless.service` and `k3s`.

* Install `k3s-rootless.service` to `~/.config/systemd/user/k3s-rootless.service`.
  Installing this file as a system-wide service (`/etc/systemd/...`) is not supported.
  Depending on the path of `k3s` binary, you might need to modify the `ExecStart=/usr/local/bin/k3s ...` line of the file.

* Run `systemctl --user daemon-reload`

* Run `systemctl --user enable --now k3s-rootless`

* Run `KUBECONFIG=~/.kube/k3s.yaml kubectl get pods -A`, and make sure the pods are running.

> **Note:** Don't try to run `k3s server --rootless` on a terminal, as it doesn't enable cgroup v2 delegation.
> If you really need to try it on a terminal, prepend `systemd-run --user -p Delegate=yes --tty` to create a systemd scope.
>
> i.e., `systemd-run --user -p Delegate=yes --tty k3s server --rootless`

### Troubleshooting

* Run `systemctl --user status k3s-rootless` to check the daemon status
* Run `journalctl --user -f -u k3s-rootless` to see the daemon log
* See also https://rootlesscontaine.rs/

## Node Labels and Taints

K3s agents can be configured with the options `--node-label` and `--node-taint` which adds a label and taint to the kubelet. The two options only add labels and/or taints [at registration time,](install/install-options/install_options.md#node-labels-and-taints-for-agents) so they can only be added once and not changed after that again by running K3s commands.

If you want to change node labels and taints after node registration you should use `kubectl`. Refer to the official Kubernetes documentation for details on how to add [taints](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) and [node labels.](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/#add-a-label-to-a-node)

## Starting the Server with the Installation Script

The installation script will auto-detect if your OS is using systemd or openrc and start the service.
When running with openrc, logs will be created at `/var/log/k3s.log`. 

When running with systemd, logs will be created in `/var/log/syslog` and viewed using `journalctl -u k3s`.

An example of installing and auto-starting with the install script:

```bash
curl -sfL https://get.k3s.io | sh -
```

When running the server manually you should get an output similar to the following:

```
$ k3s server
INFO[2019-01-22T15:16:19.908493986-07:00] Starting k3s dev                             
INFO[2019-01-22T15:16:19.908934479-07:00] Running kube-apiserver --allow-privileged=true --authorization-mode Node,RBAC --service-account-signing-key-file /var/lib/rancher/k3s/server/tls/service.key --service-cluster-ip-range 10.43.0.0/16 --advertise-port 6445 --advertise-address 127.0.0.1 --insecure-port 0 --secure-port 6444 --bind-address 127.0.0.1 --tls-cert-file /var/lib/rancher/k3s/server/tls/localhost.crt --tls-private-key-file /var/lib/rancher/k3s/server/tls/localhost.key --service-account-key-file /var/lib/rancher/k3s/server/tls/service.key --service-account-issuer k3s --api-audiences unknown --basic-auth-file /var/lib/rancher/k3s/server/cred/passwd --kubelet-client-certificate /var/lib/rancher/k3s/server/tls/token-node.crt --kubelet-client-key /var/lib/rancher/k3s/server/tls/token-node.key 
Flag --insecure-port has been deprecated, This flag will be removed in a future version.
INFO[2019-01-22T15:16:20.196766005-07:00] Running kube-scheduler --kubeconfig /var/lib/rancher/k3s/server/cred/kubeconfig-system.yaml --port 0 --secure-port 0 --leader-elect=false
INFO[2019-01-22T15:16:20.196880841-07:00] Running kube-controller-manager --kubeconfig /var/lib/rancher/k3s/server/cred/kubeconfig-system.yaml --service-account-private-key-file /var/lib/rancher/k3s/server/tls/service.key --allocate-node-cidrs --cluster-cidr 10.42.0.0/16 --root-ca-file /var/lib/rancher/k3s/server/tls/token-ca.crt --port 0 --secure-port 0 --leader-elect=false 
Flag --port has been deprecated, see --secure-port instead.
INFO[2019-01-22T15:16:20.273441984-07:00] Listening on :6443                           
INFO[2019-01-22T15:16:20.278383446-07:00] Writing manifest: /var/lib/rancher/k3s/server/manifests/coredns.yaml 
INFO[2019-01-22T15:16:20.474454524-07:00] Node token is available at /var/lib/rancher/k3s/server/node-token 
INFO[2019-01-22T15:16:20.474471391-07:00] To join node to cluster: k3s agent -s https://10.20.0.3:6443 -t ${NODE_TOKEN} 
INFO[2019-01-22T15:16:20.541027133-07:00] Wrote kubeconfig /etc/rancher/k3s/k3s.yaml
INFO[2019-01-22T15:16:20.541049100-07:00] Run: k3s kubectl                             
```

The output will likely be much longer as the agent will create a lot of logs. By default the server
will register itself as a node (run the agent).

## Additional Preparation for Alpine Linux Setup

In order to set up Alpine Linux, you have to go through the following preparation:

Update **/etc/update-extlinux.conf** by adding:

```
default_kernel_opts="...  cgroup_enable=cpuset cgroup_memory=1 cgroup_enable=memory"
```

Then update the config and reboot:

```bash
update-extlinux
reboot
```

## Running K3d (K3s in Docker) and docker-compose

[k3d](https://github.com/rancher/k3d) is a utility designed to easily run K3s in Docker.

It can be installed via the the [brew](https://brew.sh/) utility on MacOS:

```
brew install k3d
```

`rancher/k3s` images are also available to run the K3s server and agent from Docker. 

A `docker-compose.yml` is in the root of the K3s repo that serves as an example of how to run K3s from Docker. To run from `docker-compose` from this repo, run:

    docker-compose up --scale agent=3
    # kubeconfig is written to current dir

    kubectl --kubeconfig kubeconfig.yaml get node

    NAME           STATUS   ROLES    AGE   VERSION
    497278a2d6a2   Ready    <none>   11s   v1.13.2-k3s2
    d54c8b17c055   Ready    <none>   11s   v1.13.2-k3s2
    db7a5a5a5bdd   Ready    <none>   12s   v1.13.2-k3s2

To run the agent only in Docker, use `docker-compose up agent`.

Alternatively the `docker run` command can also be used:

    sudo docker run \
      -d --tmpfs /run \
      --tmpfs /var/run \
      -e K3S_URL=${SERVER_URL} \
      -e K3S_TOKEN=${NODE_TOKEN} \
      --privileged rancher/k3s:vX.Y.Z


## Enabling legacy iptables on Raspbian Buster

Raspbian Buster defaults to using `nftables` instead of `iptables`.  **K3S** networking features require `iptables` and do not work with `nftables`.  Follow the steps below to switch configure **Buster** to use `legacy iptables`:
```
sudo iptables -F
sudo update-alternatives --set iptables /usr/sbin/iptables-legacy
sudo update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy
sudo reboot
```

## Enabling cgroups for Raspbian Buster

Standard Raspbian Buster installations do not start with `cgroups` enabled. **K3S** needs `cgroups` to start the systemd service. `cgroups`can be enabled by appending `cgroup_memory=1 cgroup_enable=memory` to `/boot/cmdline.txt`.

### example of /boot/cmdline.txt
```
console=serial0,115200 console=tty1 root=PARTUUID=58b06195-02 rootfstype=ext4 elevator=deadline fsck.repair=yes rootwait cgroup_memory=1 cgroup_enable=memory
```

## SELinux Support

_Supported as of v1.19.4+k3s1. Experimental as of v1.17.4+k3s1._

If you are installing K3s on a system where SELinux is enabled by default (such as CentOS), you must ensure the proper SELinux policies have been installed. 

### Automatic Installation

_Available as of v1.19.3+k3s2_

The [install script](install/install-options/install_options.md#options-for-installation-with-script) will automatically install the SELinux RPM from the Rancher RPM repository if on a compatible system if not performing an air-gapped install. Automatic installation can be skipped by setting `INSTALL_K3S_SKIP_SELINUX_RPM=true`.

### Manual Installation

The necessary policies can be installed with the following commands:
```
yum install -y container-selinux selinux-policy-base
yum install -y https://rpm.rancher.io/k3s/latest/common/centos/7/noarch/k3s-selinux-0.2-1.el7_8.noarch.rpm
```

To force the install script to log a warning rather than fail, you can set the following environment variable: `INSTALL_K3S_SELINUX_WARN=true`.

### Enabling and Disabling SELinux Enforcement

The way that SELinux enforcement is enabled or disabled depends on the K3s version.

=== "After v1.19.1+k3s1"

  To leverage SELinux, specify the `--selinux` flag when starting K3s servers and agents.

  This option can also be specified in the K3s [configuration file:](install/install-options/install_options.md#configuration-file)

  ```
  selinux: true
  ```

  The `--disable-selinux` option should not be used. It is deprecated and will be either ignored or will be unrecognized, resulting in an error, in future minor releases.

  Using a custom `--data-dir` under SELinux is not supported. To customize it, you would most likely need to write your own custom policy. For guidance, you could refer to the [containers/container-selinux](https://github.com/containers/container-selinux) repository, which contains the SELinux policy files for Container Runtimes, and the [rancher/k3s-selinux](https://github.com/rancher/k3s-selinux) repository, which contains the SELinux policy for K3s .

=== "Before v1.19.1+k3s1"

  SELinux is automatically enabled for the built-in containerd.

  To turn off SELinux enforcement in the embedded containerd, launch K3s with the `--disable-selinux` flag.

  Using a custom `--data-dir` under SELinux is not supported. To customize it, you would most likely need to write your own custom policy. For guidance, you could refer to the [containers/container-selinux](https://github.com/containers/container-selinux) repository, which contains the SELinux policy files for Container Runtimes, and the [rancher/k3s-selinux](https://github.com/rancher/k3s-selinux) repository, which contains the SELinux policy for K3s .


## Additional preparation for (Red Hat/CentOS) Enterprise Linux

It is recommended to turn off firewalld:
```
systemctl disable firewalld --now
```

If enabled, it is required to disable nm-cloud-setup and reboot the node:
```
systemctl disable nm-cloud-setup.service nm-cloud-setup.timer
reboot
```

## Enabling Lazy Pulling of eStargz (Experimental)

### What's lazy pulling and eStargz?

Pulling images is known as one of the time-consuming steps in the container lifecycle.
According to [Harter, et al.](https://www.usenix.org/conference/fast16/technical-sessions/presentation/harter),

> pulling packages accounts for 76% of container start time, but only 6.4% of that data is read

To address this issue, k3s experimentally supports *lazy pulling* of image contents.
This allows k3s to start a container before the entire image has been pulled.
Instead, the necessary chunks of contents (e.g. individual files) are fetched on-demand. 
Especially for large images, this technique can shorten the container startup latency.

To enable lazy pulling, the target image needs to be formatted as [*eStargz*](https://github.com/containerd/stargz-snapshotter/blob/main/docs/stargz-estargz.md).
This is an OCI-alternative but 100% OCI-compatible image format for lazy pulling.
Because of the compatibility, eStargz can be pushed to standard container registries (e.g. ghcr.io) as well as this is *still runnable* even on eStargz-agnostic runtimes.

eStargz is developed based on the [stargz format proposed by Google CRFS project](https://github.com/google/crfs) but comes with practical features including content verification and performance optimization.
For more details about lazy pulling and eStargz, please refer to [Stargz Snapshotter project repository](https://github.com/containerd/stargz-snapshotter).

### Configure k3s for lazy pulling of eStargz

As shown in the following, `--snapshotter=stargz` option is needed for k3s server and agent.

```
k3s server --snapshotter=stargz
```

With this configuration, you can perform lazy pulling for eStargz-formatted images.
The following Pod manifest uses eStargz-formatted `node:13.13.0` image (`ghcr.io/stargz-containers/node:13.13.0-esgz`).
k3s performs lazy pulling for this image.

```
apiVersion: v1
kind: Pod
metadata:
  name: nodejs
spec:
  containers:
  - name: nodejs-estargz
    image: ghcr.io/stargz-containers/node:13.13.0-esgz
    command: ["node"]
    args:
    - -e
    - var http = require('http');
      http.createServer(function(req, res) {
        res.writeHead(200);
        res.end('Hello World!\n');
      }).listen(80);
    ports:
    - containerPort: 80
```

## Additional Logging Sources

[Rancher logging](https://rancher.com/docs//rancher/v2.6/en/logging/helm-chart-options/) for K3s can be installed without using Rancher. The following instructions should be executed to do so:

```
helm repo add rancher-charts https://charts.rancher.io
helm repo update
helm install --create-namespace -n cattle-logging-system rancher-logging-crd rancher-charts/rancher-logging-crd
helm install --create-namespace -n cattle-logging-system rancher-logging --set additionalLoggingSources.k3s.enabled=true rancher-charts/rancher-logging
```

## Server and agent tokens

In K3s, there are two types of tokens: K3S_TOKEN and K3S_AGENT_TOKEN.

K3S_TOKEN: Defines the key required by the server to offer the HTTP config resources. These resources are requested by the other servers before joining the K3s HA cluster. If the K3S_AGENT_TOKEN is not defined, the agents use this token as well to access the required HTTP resources to join the cluster. Note that this token is also used to generate the encryption key for important content in the database (e.g., bootstrap data).

K3S_AGENT_TOKEN: Optional. Defines the key required by the server to offer HTTP config resources to the agents. If not defined, agents will require K3S_TOKEN. Defining K3S_AGENT_TOKEN is encouraged to avoid agents having to know K3S_TOKEN, which is also used to encrypt data.

If no K3S_TOKEN is defined, the first K3s server will generate a random one. The result is part of the content in `/var/lib/rancher/k3s/server/token`. For example, `K1070878408e06a827960208f84ed18b65fa10f27864e71a57d9e053c4caff8504b::server:df54383b5659b9280aa1e73e60ef78fc`, where `df54383b5659b9280aa1e73e60ef78fc` is the K3S_TOKEN.