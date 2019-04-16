k3s - 5 less than k8s
===============================================

Lightweight Kubernetes.  Easy to install, half the memory, all in a binary less than 40mb.

Great for
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

Quick start
-----------
1. Download `k3s` from latest [release](https://github.com/rancher/k3s/releases/latest), x86_64, armhf, and arm64 are
   supported
2. Run server

```bash
sudo k3s server &
# Kubeconfig is written to /etc/rancher/k3s/k3s.yaml
sudo k3s kubectl get node

# On a different node run the below. NODE_TOKEN comes from /var/lib/rancher/k3s/server/node-token
# on your server
sudo k3s agent --server https://myserver:6443 --token ${NODE_TOKEN}

```

Running server
--------------

To run the server just do

    k3s server

You should get an output similar to

```
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

The output will probably be much longer as the agent will spew a lot of logs. By default the server
will register itself as a node (run the agent).  It is common and almost required these days
that the control plane be part of the cluster.  To not run the agent by default use the `--disable-agent`
flag

    k3s server --disable-agent

At this point, you can run the agent as a separate process or not run it on this node at all.

If you encounter an error like `"stream server error: listen tcp: lookup some-host on X.X.X.X:53: no such host"`
when starting k3s please ensure `/etc/hosts` contains your current hostname (output of `hostname`), 
set to a 127.x.x.x address. For example:
```
127.0.1.1	myhost
```

Joining nodes
-------------

When the server starts it creates a file `/var/lib/rancher/k3s/server/node-token`. Use the contents
of that file as `NODE_TOKEN` and then run the agent as follows

    k3s agent --server https://myserver:6443 --token ${NODE_TOKEN}

That's it.

Accessing cluster from outside
-----------------------------

Copy `/etc/rancher/k3s/k3s.yaml` on your machine located outside the cluster as `~/.kube/config`. Then replace
"localhost" with the IP or name of your k3s server. `kubectl` can now manage your k3s cluster.

Auto-deploying manifests
------------------------

Any file found in `/var/lib/rancher/k3s/server/manifests` will automatically be deployed to
Kubernetes in a manner similar to `kubectl apply`.

It is also possible to deploy Helm charts. k3s supports a CRD controller for installing charts. A YAML file specification can look as following (example taken from `/var/lib/rancher/k3s/server/manifests/traefik.yaml`):

```yaml
apiVersion: k3s.cattle.io/v1
kind: HelmChart
metadata:
  name: traefik
  namespace: kube-system
spec:
  chart: stable/traefik
  set:
    rbac.enabled: "true"
    ssl.enabled: "true"
```

Keep in mind that `namespace` in your HelmChart resource metadata section should always be `kube-system`, because k3s deploy controller is configured to watch this namespace for new HelmChart resources. If you want to specify the namespace for the actual helm release, you can do that using `targetNamespace` key in the spec section:

```
apiVersion: k3s.cattle.io/v1
kind: HelmChart
metadata:
  name: grafana
  namespace: kube-system
spec:
  chart: stable/grafana
  targetNamespace: monitoring
  set:
    adminPassword: "NotVerySafePassword"
  valuesContent: |-
    image:
      tag: master
    env:
      GF_EXPLORE_ENABLED: true
    adminUser: admin
    sidecar:
      datasources:
        enabled: true
```

Also note that besides `set` you can use `valuesContent` in the spec section. And it's okay to use both of them.

Building from source
--------------------

The clone will be much faster on this repo if you do

    git clone --depth 1 https://github.com/rancher/k3s.git

This repo includes all of Kubernetes history so `--depth 1` will avoid most of that.

For development, you just need go 1.11 and a sane GOPATH.  To compile the binaries run

```bash
go build -o k3s
go build -o kubectl ./cmd/kubectl
go build -o hyperkube ./vendor/k8s.io/kubernetes/cmd/hyperkube
```

This will create the main executable, but it does not include the dependencies like containerd, CNI,
etc.  To run a server and agent with all the dependencies for development run the following
helper scripts

```bash
# Server
./scripts/dev-server.sh

# Agent
./scripts/dev-agent.sh
```

To build the full release binary run `make` and that will create `./dist/artifacts/k3s`

Uninstalling server
-----------------

If you installed your k3s server with the help of `install.sh` script from the root directory, you may use the uninstall script generated during installation, which will be created on your server node at `/usr/local/bin/k3s-uninstall.sh`

Kubernetes source
-----------------

The source code for Kubernetes is in `vendor/` and the location from which that is copied
is in `./vendor.conf`.  Go to the referenced repo/tag and you'll find all the patches applied
to upstream Kubernetes.

Open ports / Network security
---------------------------

The server needs port 6443 to be accessible by the nodes.  The nodes need to be able to reach
other nodes over UDP port 8472.  This is used for flannel VXLAN.  If you don't use flannel
and provide your own custom CNI, then 8472 is not needed by k3s. The node should not listen
on any other port.  k3s uses reverse tunneling such that the nodes make outbound connections
to the server and all kubelet traffic runs through that tunnel.

IMPORTANT. The VXLAN port on nodes should not be exposed to the world, it opens up your
cluster network to accessed by anyone.  Run your nodes behind a firewall/security group that
disables access to port 8472.


Server HA
---------
Just don't right now :)  It's currently broken.


Running in Docker (and docker-compose)
-----------------

I wouldn't be me if I couldn't run my cluster in Docker.  `rancher/k3s` images are available
to run k3s server and agent from Docker.  A `docker-compose.yml` is in the root of this repo that
serves as an example of how to run k3s from Docker.  To run from `docker-compose` from this repo run

    docker-compose up --scale node=3
    # kubeconfig is written to current dir
    kubectl --kubeconfig kubeconfig.yaml get node

    NAME           STATUS   ROLES    AGE   VERSION
    497278a2d6a2   Ready    <none>   11s   v1.13.2-k3s2
    d54c8b17c055   Ready    <none>   11s   v1.13.2-k3s2
    db7a5a5a5bdd   Ready    <none>   12s   v1.13.2-k3s2

To run the agent only in Docker use the following `docker-compose-agent.yml` is in the root of this repo that
serves as an example of how to run k3s agent from Docker. Alternatively the Docker run command can also be used;

    sudo docker run -d --tmpfs /run --tmpfs /var/run -e K3S_URL=${SERVER_URL} -e K3S_TOKEN=${NODE_TOKEN} --privileged rancher/k3s:v0.4.0

    sudo docker run -d --tmpfs /run --tmpfs /var/run -e K3S_URL=https://k3s.example.com:6443 -e K3S_TOKEN=K13849a67fc385fd3c0fa6133a8649d9e717b0258b3b09c87ffc33dae362c12d8c0::node:2e373dca319a0525745fd8b3d8120d9c --privileged rancher/k3s:v0.4.0


Hyperkube
--------

k3s is bundled in a nice wrapper to remove the majority of the headache of running k8s. If
you don't want that wrapper and just want a smaller k8s distro, the releases includes
the `hyperkube` binary you can use.  It's then up to you to know how to use `hyperkube`. If
you want individual binaries you will need to compile them yourself from source

containerd and Docker
----------

k3s includes and defaults to containerd. Why? Because it's just plain better. If you want to
run with Docker first stop and think, "Really? Do I really want more headache?" If still
yes then you just need to run the agent with the `--docker` flag

     k3s agent -s ${SERVER_URL} -t ${NODE_TOKEN} --docker &

systemd
-------

If you are bound by the shackles of systemd (as most of us are), there is a sample unit file
in the root of this repo `k3s.service` which is as follows

```ini
[Unit]
Description=Lightweight Kubernetes
Documentation=https://k3s.io
After=network.target

[Service]
Type=notify
EnvironmentFile=/etc/systemd/system/k3s.service.env
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/k3s server
KillMode=process
Delegate=yes
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=infinity

[Install]
WantedBy=multi-user.target
```

The k3s `install.sh` script also provides a convenient way for installing to systemd,
to install the agent and server as a k3s service just run:
```sh
curl -sfL https://get.k3s.io | sh -
```

The install script will attempt to download the latest release, to specify a specific
version for download we can use the `INSTALL_K3S_VERSION` environment variable, eg:
```sh
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=vX.Y.Z-rc1 sh -
```

To install just the server without an agent we can add a `INSTALL_K3S_EXEC`
environment variable to the command:
```sh
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable-agent" sh -
```

To install just the agent without a server we should pass `K3S_URL` along with
`K3S_TOKEN` or `K3S_CLUSTER_SECRET`, eg:
```sh
curl -sfL https://get.k3s.io | K3S_URL=https://example-url:6443 K3S_TOKEN=XXX sh -
```

The installer can also be run without performing downloads by setting `INSTALL_K3S_SKIP_DOWNLOAD=true`, eg:
```sh
curl -sfL https://github.com/rancher/k3s/releases/download/vX.Y.Z/k3s -o /usr/local/bin/k3s
chmod 0755 /usr/local/bin/k3s

curl -sfL https://get.k3s.io -o install-k3s.sh
chmod 0755 install-k3s.sh

export INSTALL_K3S_SKIP_DOWNLOAD=true
./install-k3s.sh
```

The full help text for the install script environment variables are as follows:
   - `K3S_*`

     Environment variables which begin with `K3S_` will be preserved for the
     systemd service to use. Setting `K3S_URL` without explicitly setting
     a systemd exec command will default the command to "agent", and we
     enforce that `K3S_TOKEN` or `K3S_CLUSTER_SECRET` is also set.

   - `INSTALL_K3S_SKIP_DOWNLOAD`

     If set to true will not download k3s hash or binary.

   - `INSTALL_K3S_VERSION`

     Version of k3s to download from github. Will attempt to download the
     latest version if not specified.

   - `INSTALL_K3S_BIN_DIR`

     Directory to install k3s binary, links, and uninstall script to, or use
     /usr/local/bin as the default

   - `INSTALL_K3S_SYSTEMD_DIR`

     Directory to install systemd service and environment files to, or use
     /etc/systemd/system as the default

   - `INSTALL_K3S_EXEC` or script arguments

     Command with flags to use for launching k3s in the systemd service, if
     the command is not specified will default to "agent" if `K3S_URL` is set
     or "server" if not. The final systemd command resolves to a combination
     of EXEC and script args ($@).

     The following commands result in the same behavior:
     ```sh
     curl ... | INSTALL_K3S_EXEC="--disable-agent" sh -s -
     curl ... | INSTALL_K3S_EXEC="server --disable-agent" sh -s -
     curl ... | INSTALL_K3S_EXEC="server" sh -s - --disable-agent
     curl ... | sh -s - server --disable-agent
     curl ... | sh -s - --disable-agent
     ```

   - `INSTALL_K3S_NAME`

     Name of systemd service to create, will default from the k3s exec command
     if not specified. If specified the name will be prefixed with 'k3s-'.

   - `INSTALL_K3S_TYPE`

     Type of systemd service to create, will default from the k3s exec command
     if not specified.


Flannel
-------

Flannel is included by default, if you don't want flannel then run the agent with `--no-flannel` as follows

     k3s agent -u ${SERVER_URL} -t ${NODE_TOKEN} --no-flannel &

In this setup you will still be required to install your own CNI driver.  More info [here](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/#pod-network)

CoreDNS
-------

CoreDNS is deployed on start of the agent, to disable add `--no-deploy coredns` to the server

     k3s server --no-deploy coredns

If you don't install CoreDNS you will need to install a cluster DNS provider yourself.

Traefik
-------

Traefik is deployed by default when starting the server; to disable it, start the server with `--no-deploy traefik` like this

     k3s server --no-deploy traefik

Service load balancer
---------------------

k3s includes a basic service load balancer that uses available host ports.  If you try to create
a load balancer that listens on port 80, for example, it will try to find a free host in the cluster
for port 80.  If no port is available the load balancer will stay in Pending.

To disable the embedded service load balancer (if you wish to use a different implementation like
MetalLB) just add `--no-deploy=servicelb` to the server on startup.

Air-Gap Support
---------------

k3s supports pre-loading of containerd images by placing them in the `images` directory for the agent before starting, eg:
```sh
sudo mkdir -p /var/lib/rancher/k3s/agent/images/
sudo cp ./k3s-airgap-images-$ARCH.tar /var/lib/rancher/k3s/agent/images/
```
Images needed for a base install are provided through the releases page, additional images can be created with the `docker save` command.

Offline Helm charts are served from the `/var/lib/rancher/k3s/server/static` directory, and Helm chart manifests may reference the static files with a `%{KUBERNETES_API}%` templated variable. For example, the default traefik manifest chart installs from `https://%{KUBERNETES_API}%/static/charts/traefik-X.Y.Z.tgz`.

If networking is completely disabled k3s may not be able to start (ie ethernet unplugged or wifi disconnected), in which case it may be necessary to add a default route. For example:
```sh
sudo ip -c address add 192.168.123.123/24 dev eno1
sudo ip route add default via 192.168.123.1
```

k3s additionally provides a `--resolv-conf` flag for kubelets, which may help with configuring DNS in air-gap networks.

Rootless - (Some advanced magic, user beware)
--------

Initial rootless support has been added but there are a series of significant usability issues surrounding it.
We are releasing the initial support for those interested in rootless and hopefully some people can help to
improve the usability.  First ensure you have proper setup and support for user namespaces.  Refer to the
[requirements section](https://github.com/rootless-containers/rootlesskit#setup) in rootlesskit for instructions.
In short, latest Ubuntu is your best bet for this to work.

## Issues w/ Rootless

When running rootless a new network namespace is created.  This means that k3s instance is running with networking
fairly detached from the host.  The only way to access services run in k3s from the host is to setup port forwards
to the k3s network namespace.  We have a controller that will automatically bind 6443 and any service port to the
host with an offset of 10000.  That means service port 80 will become 10080 on the host.  Once you kill k3s and then
start a new instance of k3s it will create a new network namespace, but it doesn't kill the old pods.  So you are left
with a fairly broken setup.  This is the main issue at the moment, how to deal with the network namespace.

## Running w/ Rootless

Just add `--rootless` flag to either server or agent.  So run `k3s server --rootless` and then look for the message
`Wrote kubeconfig [SOME PATH]` for where your kubeconfig to access you cluster is.  Becareful, if you use `-o` to write
the kubeconfig to a different directory it will probably not work.  This is because the k3s instance in running in a different
mount namespace.

TODO
----
Currently broken or stuff that needs to be done for this to be considered production quality.

1. Metrics API ([fixed](https://github.com/rancher/k3s/issues/252): use `k3s server --kubelet-arg="address=0.0.0.0"` and apply `recipes/metrics-server`)
2. HA
3. Work on e2e, sonobouy.
4. etcd doesn't actually work because args aren't exposed
