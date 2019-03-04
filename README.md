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
    
Quick Start
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

Running Server
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

Joining Nodes
-------------

When the server starts it creates a file `/var/lib/rancher/k3s/server/node-token`. Use the contents
of that file as `NODE_TOKEN` and then run the agent as follows

    k3s agent --server https://myserver:6443 --token ${NODE_TOKEN}
    
That's it.

Accessing cluster from outside
-----------------------------

Copy the file `/etc/rancher/k3s/k3s.yaml` on your machine located outside the cluster as `~/.kube/config`. Then edit it and replace
"localhost" with the IP or name of the k3s server. Now you can use `kubectl` to manage your k3s cluster.

Auto-deploying manifests
------------------------

Any file found in `/var/lib/rancher/k3s/server/manifests` will automatically be deployed to
Kubernetes in a manner similar to `kubectl apply`.

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

To build the full release binary run `make` and that will create `./dist/k3s`

Kubernetes Source
-----------------

The source code for Kubernetes is in `vendor/` and the location from which that is copied
is in `./vendor.conf`.  Go to the referenced repo/tag and you'll find all the patches applied
to upstream Kubernetes.

Open Ports/Network Security
---------------------------

The server needs port 6443 to be accessible by the nodes.  The nodes need to be able to reach
other nodes over UDP port 4789.  This is used for flannel VXLAN.  If you don't use flannel
and provide your own custom CNI, then 4789 is not needed by k3s. The node should not listen
on any other port.  k3s uses reverse tunneling such that the nodes make outbound connections
to the server and all kubelet traffic runs through that tunnel.

IMPORTANT. The VXLAN port on nodes should not be exposed to the world, it opens up your
cluster network to accessed by anyone.  Run your nodes behind a firewall/security group that
disables access to port 4789.


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

     k3s agent -u ${SERVER_URL} -t ${NODE_TOKEN} --docker &
     
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
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/k3s server
KillMode=process
Delegate=yes
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity

[Install]
WantedBy=multi-user.target
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

Service Load Balancer
---------------------

k3s includes a basic service load balancer that uses available host ports.  If you try to create
a load balancer that listens on port 80, for example, it will try to find a free host in the cluster
for port 80.  If no port is available the load balancer will stay in Pending.

To disable the embedded service load balancer (if you wish to use a different implementation like
MetalLB) just add `--no-deploy=servicelb` to the server on startup.

TODO
----
Currently broken or stuff that needs to be done for this to be considered production quality.

1. Metrics API due to kube aggregation not being setup
2. HA
3. Work on e2e, sonobouy.
4. etcd doesn't actually work because args aren't exposed
    
