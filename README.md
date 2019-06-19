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
apiVersion: helm.cattle.io/v1
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
apiVersion: helm.cattle.io/v1
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

k3s versions <= v0.5.0 used `k3s.cattle.io` for the api group of helmcharts, this has been changed to `helm.cattle.io` for later versions.

Storage Backends
----------------

As of version 0.6.0, k3s can support various storage backends including: SQLite (default), MySQL, Postgres, and etcd, this enahancement depends on the following arguments that can be passed to k3s server:

```
--storage-backend value             Specify storage type etcd3 or kvsql [$K3S_STORAGE_BACKEND]
--storage-endpoint value            Specify etcd, Mysql, Postgres, or Sqlite (default) data source name [$K3S_STORAGE_ENDPOINT]
--storage-cafile value              SSL Certificate Authority file used to secure storage backend communication [$K3S_STORAGE_CAFILE]
--storage-certfile value            SSL certification file used to secure storage backend communication [$K3S_STORAGE_CERTFILE]
--storage-keyfile value             SSL key file used to secure storage backend communication [$K3S_STORAGE_KEYFILE]
```

## MySQL

To use k3s with MySQL storage backend, you can specify the following for insecure connection:

```
k3s server --storage-endpoint="mysql://"
```
By default the server will attempt to connect to mysql using the mysql socket at `/var/run/mysqld/mysqld.sock` using the root user and with no password, k3s will also create a database with the name `kubernetes` if the database is not specified in the DSN.

To override the method of connection, user/pass, and database name, you can provide a custom DSN, for example:

```
k3s server --storage-endpoint="mysql://k3suser:k3spass@tcp(192.168.1.100:3306)/k3stest"
```

This command will attempt to connect to MySQL on host `192.168.1.100` on port `3306` with username `k3suser` and password `k3spass` and k3s will automatically create a new database with the name `k3stest` if it doesn't exist, for more information about the MySQL driver data source name, please refer to https://github.com/go-sql-driver/mysql#dsn-data-source-name

To connect to MySQL securely, you can use the following example:
```
k3s server --storage-endpoint="mysql://k3suser:k3spass@tcp(192.168.1.100:3306)/k3stest" --storage-cafile ca.crt --storage-certfile mysql.crt --storage-keyfile mysql.key
```
The above command will use these certificates to generate the tls config to communicate with mysql securely.


## Postgres

Connection to postgres can be established using the following command:

```
k3s server --storage-endpoint="postgres://"
```

By default the server will attempt to connect to postgres on localhost with using the `postgres` user and with `postgres` password, k3s will also create a database with the name `kubernetes` if the database is not specified in the DSN.

To override the method of connection, user/pass, and database name, you can provide a custom DSN, for example:

```
k3s server --storage-endpoint="postgres://k3suser:k3spass@192.168.1.100:5432/k3stest"
```

This command will attempt to connect to Postgres on host `192.168.1.100` on port `5432` with username `k3suser` and password `k3spass` and k3s will automatically create a new database with the name `k3stest` if it doesn't exist, for more information about the Postgres driver data source name, please refer to https://godoc.org/github.com/lib/pq

To connect to Postgres securely, you can use the following example:

```
k3s server --storage-endpoint="postgres://k3suser:k3spass@192.168.1.100:5432/k3stest?sslmode=verify-full" --storage-certfile postgres.crt --storage-keyfile postgres.key --storage-cafile ca.crt
```

The above command will use these certificates to generate the tls config to communicate with postgres securely, note that the `sslmode` in the example is `verify-full` which verify that the certification presented by the server was signed by a trusted CA and the server host name matches the one in the certificate.

## etcd

Connection to postgres can be established using the following command:

```
k3s server --storage-backend=etcd3 --storage-endpoint="https://127.0.0.1:2379"
```
The above command will attempt to connect insecurely to etcd on localhost with port `2379`, you can connect securely to etcd using the following command:

```
k3s server --storage-backend=etcd3 --storage-endpoint="https://127.0.0.1:2379" --storage-cafile ca.crt --storage-certfile etcd.crt --storage-keyfile etcd.key
```

Building from source
--------------------

The clone will be much faster on this repo if you do

    git clone --depth 1 https://github.com/rancher/k3s.git

This repo includes all of Kubernetes history so `--depth 1` will avoid most of that.

For development, you just need go 1.12 and a sane GOPATH.  To compile the binaries run

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


Customizing components
----------------------

As of v0.3.0 any of the following processes can be customized with extra flags:

- kube-apiserver (server)
- kube-controller-manager (server)
- kube-scheduler (server)
- kubelet (agent)
- kube-proxy (agent)

Adding extra argument can be done by passing the following flags to server or agent:
```
--kube-apiserver-arg value
--kube-scheduler-arg value
--kube-controller-arg value
--kubelet-arg value        
--kube-proxy-arg value     
```
For example to add the following arguments `-v=9` and `log-file=/tmp/kubeapi.log` to the kube-apiserver, you should pass the following:
```
k3s server --kube-apiserver-arg v=9 --kube-apiserver-arg log-file=/tmp/kubeapi.log
```

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

    sudo docker run -d --tmpfs /run --tmpfs /var/run -e K3S_URL=${SERVER_URL} -e K3S_TOKEN=${NODE_TOKEN} --privileged rancher/k3s:v0.6.0

    sudo docker run -d --tmpfs /run --tmpfs /var/run -e K3S_URL=https://k3s.example.com:6443 -e K3S_TOKEN=K13849a67fc385fd3c0fa6133a8649d9e717b0258b3b09c87ffc33dae362c12d8c0::node:2e373dca319a0525745fd8b3d8120d9c --privileged rancher/k3s:v0.6.0


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

k3s will generate config.toml for containerd in `/var/lib/rancher/k3s/agent/etc/containerd/config.toml`, for advanced customization for this file you can create another file called `config.toml.tmpl` in the same directory and it will be used instead.

The `config.toml.tmpl` will be treated as a Golang template file, and the `config.Node` structure is being passed to the template,the following is an example on how to use the structure to customize the configuration file https://github.com/rancher/k3s/blob/master/pkg/agent/templates/templates.go#L16-L32

systemd
-------

If you are bound by the shackles of systemd (as most of us are), there is a sample unit file
in the root of this repo `k3s.service` which is as follows

```ini
[Unit]
Description=Lightweight Kubernetes
Documentation=https://k3s.io
After=network-online.target

[Service]
Type=notify
EnvironmentFile=/etc/systemd/system/k3s.service.env
ExecStart=/usr/local/bin/k3s server
KillMode=process
Delegate=yes
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0
Restart=always

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

openrc on Alpine Linux
-------

In order to pre-setup Alpine Linux you have to go through the following steps:

```bash
echo "cgroup /sys/fs/cgroup cgroup defaults 0 0" >> /etc/fstab

cat >> /etc/cgconfig.conf <<EOF
mount {
cpuacct = /cgroup/cpuacct;
memory = /cgroup/memory;
devices = /cgroup/devices;
freezer = /cgroup/freezer;
net_cls = /cgroup/net_cls;
blkio = /cgroup/blkio;
cpuset = /cgroup/cpuset;
cpu = /cgroup/cpu;
}
EOF
```

Then update **/etc/update-extlinux.conf** by adding:

```
default_kernel_opts="...  cgroup_enable=cpuset cgroup_memory=1 cgroup_enable=memory"
```

Than update the config and reboot

```bash
update-extlinux
reboot
```

After rebooting:

- download **k3s** to **/usr/local/bin/k3s**
- create an openrc file in **/etc/init.d**

For the server:

```bash
#!/sbin/openrc-run

command=/usr/local/bin/k3s
command_args="server"
pidfile=

name="k3s"
description="Lightweight Kubernetes"
```

For the agent:

```bash
#!/sbin/openrc-run

command=/usr/local/bin/k3s
command_args="agent --server https://myserver:6443 --token ${NODE_TOKEN}"
pidfile=

name="k3s"
description="Lightweight Kubernetes"
```

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

Node Labels and Taints
----------------------

k3s server and agent can be configured with options `--node-label` and `--node-taint` which adds set of Labels and Taints to kubelet, the two options only adds labels/taints at registration time, so they can only be added once and not changed after that, an example to add new label is:
```
k3s server --node-label foo=bar --node-label hello=world --node-taint key1=value1:NoExecute
```

## Issues w/ Rootless

### Ports
When running rootless a new network namespace is created.  This means that k3s instance is running with networking
fairly detached from the host.  The only way to access services run in k3s from the host is to setup port forwards
to the k3s network namespace.  We have a controller that will automatically bind 6443 and service port below 1024 to the host with an offset of 10000. 

That means service port 80 will become 10080 on the host, but 8080 will become 8080 without any offset.

Currently, only `LoadBalancer` services are automatically bound.

### Daemon lifecycle
Once you kill k3s and then start a new instance of k3s it will create a new network namespace, but it doesn't kill the old pods.  So you are left
with a fairly broken setup.  This is the main issue at the moment, how to deal with the network namespace.

The issue is tracked in https://github.com/rootless-containers/rootlesskit/issues/65

### Cgroups

Cgroups are not supported

## Running w/ Rootless

Just add `--rootless` flag to either server or agent.  So run `k3s server --rootless` and then look for the message
`Wrote kubeconfig [SOME PATH]` for where your kubeconfig to access you cluster is.  Becareful, if you use `-o` to write
the kubeconfig to a different directory it will probably not work.  This is because the k3s instance in running in a different
mount namespace.

## Upgrades

To upgrade k3s from an older version you can re-run the installation script using the same flags, eg:

```sh
curl -sfL https://get.k3s.io | sh -
```

If you want to upgrade to specific version you can run the following command:

```sh
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=vX.Y.Z-rc1 sh -
```

Or to manually upgrade k3s:
1. Download the desired version of k3s from [releases](https://github.com/rancher/k3s/releases/latest)
2. Install to an appropriate location (normally `/usr/local/bin/k3s`)
3. Stop the old version
4. Start the new version

Restarting k3s is supported by the installation script for systemd and openrc.
To restart manually for systemd use:
```sh
sudo systemctl restart k3s
```

To restart manually for openrc use:
```sh
sudo service k3s restart
```

Upgrading an air-gap environment can be accomplished in the following manner:
1. Download air-gap images and install if changed
2. Install new k3s binary (from installer or manual download)
3. Restart k3s (if not restarted automatically by installer)

TODO
----
Current items to implement before this is to be considered production quality.

1. Multi-Server / High Availability (HA)
2. Documentation moved to Rancher
3. Automated tests for k3s specific features
