
# Disable Components Flags

When starting K3s server with --cluster-init it will run all control plane components that includes (api server, controller manager, scheduler, and etcd). However you can run server nodes with certain components and execlude others, the following sections will explain how to do that.

## ETCD Only Nodes

This document assumes you run K3s server with embedded etcd by passing `--cluster-init` flag to the server process.

To run a K3s server with only etcd components you can pass `--disable-apiserver --disable-controller-manager --disable-scheduler` flags to k3s, this will result in running a server node with only etcd, for example to run K3s server with those flags:

```
curl -fL https://get.k3s.io | sh -s - server --cluster-init --disable-apiserver --disable-controller-manager --disable-scheduler
```

You can join other nodes to the cluster normally after that.

## Disable ETCD

You can also disable etcd from a server node and this will result in a k3s server running control components other than etcd, that can be accomplished by running k3s server with flag `--disable-etcd` for example to join another node with only control components to the etcd node created in the previous section:

```
curl -fL https://get.k3s.io | sh -s - server --token <token> --disable-etcd --server https://<etcd-only-node>:6443 
```

The end result will be a two nodes one of them is etcd only node and the other one is controlplane only node, if you check the node list you should see something like the following:

```
kubectl get nodes
NAME              STATUS   ROLES                       AGE     VERSION
ip-172-31-13-32   Ready    etcd                        5h39m   v1.20.4+k3s1
ip-172-31-14-69   Ready    control-plane,master        5h39m   v1.20.4+k3s1
```

Note that you can run `kubectl` commands only on the k3s server that has the api running, and you can't run `kubectl` commands on etcd only nodes.


### Re-enabling control components

In both cases you can re-enable any component that you already disabled simply by removing the corresponding flag that disables them, so for example if you want to revert the etcd only node back to a full k3s server with all components you can just remove the following 3 flags `--disable-apiserver --disable-controller-manager --disable-scheduler`, so in our example to revert back node `ip-172-31-13-32` to a full k3s server you can just re-run the curl command without the disable flags:
```
curl -fL https://get.k3s.io | sh -s - server --cluster-init
``` 

you will notice that all components started again and you can run kubectl commands again:

```
kubectl get nodes
NAME              STATUS   ROLES                       AGE     VERSION
ip-172-31-13-32   Ready    control-plane,etcd,master   5h45m   v1.20.4+k3s1
ip-172-31-14-69   Ready    control-plane,master        5h45m   v1.20.4+k3s1
```

Notice that role labels has been re-added to the node `ip-172-31-13-32` with the correct labels (control-plane,etcd,master).

## Add disable flags using the config file

In any of the previous situations you can use the config file instead of running the curl commands with the associated flags, for example to run an etcd only node you can add the following options to the `/etc/rancher/k3s/config.yaml` file:

```
---
disable-apiserver: true
disable-controller-manager: true
disable-scheduler: true
cluster-init: true
```
and then start K3s using the curl command without any arguents:

```
curl -fL https://get.k3s.io | sh -
```
## Disable components using .skip files

For any yaml file under `/var/lib/rancher/k3s/server/manifests` (coredns, traefik, local-storeage, etc.) you can add a `.skip` file which will cause K3s to not apply the associated yaml file.
For example, adding `traefik.yaml.skip` in the manifests directory will cause K3s to skip `traefik.yaml`.
```
ls /var/lib/rancher/k3s/server/manifests
ccm.yaml      local-storage.yaml  rolebindings.yaml  traefik.yaml.skip
coredns.yaml  traefik.yaml

kubectl get pods -A
NAMESPACE     NAME                                     READY   STATUS    RESTARTS   AGE
kube-system   local-path-provisioner-64ffb68fd-xx98j   1/1     Running   0          74s
kube-system   metrics-server-5489f84d5d-7zwkt          1/1     Running   0          74s
kube-system   coredns-85cb69466-vcq7j                  1/1     Running   0          74s
```
