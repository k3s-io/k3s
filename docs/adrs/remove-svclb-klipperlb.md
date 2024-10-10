# Remove svclb klipper-lb

Date: 2024-09-26

## Status

Discussing

## Context

There are three types of services in Kubernetes:
* ClusterIP
* NodePort
* LoadBalancer

If we want to expose the service to external clients, i.e. clients outside of the Kubernetes cluster, we need to use NodePort or Loadbalancer types. The latter uses an externalIP, normally a publicIP, which can be easily reached from external clients. To support Loadbalancer service types, an external controller (loadbalancer controller) is required.

The loadbalancer controller takes care of three tasks:
1 - Watches the kube-api for services of type LoadBalancer
2 - Sets up the infrastructure to provide the connectivity (externalIP ==> service)
3 - Sets the externalIP

K3s embeds a simple [loadbalancer controller](https://github.com/k3s-io/k3s/tree/master/pkg/cloudprovider) that we call svclb, which has been part of K3s since its inception. When a new service of type LoadBalancer comes up, this svclb [creates a daemonset](https://github.com/k3s-io/k3s/blob/master/pkg/cloudprovider/loadbalancer.go#L35). That daemonset uses [hostPort](https://github.com/k3s-io/k3s/blob/master/pkg/cloudprovider/servicelb.go#L526-L531) to reserve the service port in all nodes. Subsequently, the serviceLB controller queries the daemonset pods [to know the node ips](https://github.com/k3s-io/k3s/blob/master/pkg/cloudprovider/servicelb.go#L291) and sets those node ips as [the externalIPs for the service](https://github.com/k3s-io/k3s/blob/master/pkg/cloudprovider/servicelb.go#L299)

When an external client wants to reach the service, it needs to point to any of the node ips and use the service port. The flow of traffic would be the following:
1 - Traffic reaches the node
2 - Because hostport is reserving the service port in the node, traffic is forwarded to the daemonset pod
3 - The daemonset pod, [using klipper-lb image](https://github.com/k3s-io/klipper-lb), applies some iptables magic which replaces the destination IP with the clusterIP of the desired service
4 - Traffic gets routed to the service using regular kubernetes networking

However, after some investigation, it was found that traffic is never reaching the daemonset pod. The reason for this is that when a service gets an externalIP, kube-proxy reacts to this and adds a new rule in iptables chain `KUBE-SERVICES`. This rule also replaces the destination IP with the clusterIP of the desired service. Moreover, the `KUBE-SERVICES` chain comes before the hostPort logic and hence this is the path the traffic takes.

EXAMPLE:

Imagine a two node cluster. The traefik service uses type LoadBalancer for two ports: 80 and 443. It gets 4 external ips (2 IPv4 and 2 IPv6) 
```
NAMESPACE     NAME             TYPE           CLUSTER-IP      EXTERNAL-IP                                                         PORT(S)                      AGE
default       kubernetes       ClusterIP      10.43.0.1       <none>                                                              443/TCP                      56m
kube-system   kube-dns         ClusterIP      10.43.0.10      <none>                                                              53/UDP,53/TCP,9153/TCP       56m
kube-system   metrics-server   ClusterIP      10.43.55.117    <none>                                                              443/TCP                      56m
kube-system   traefik          LoadBalancer   10.43.206.216   10.1.1.13,10.1.1.16,fd56:5da5:a285:eea0::6,fd56:5da5:a285:eea0::8   80:30235/TCP,443:32373/TCP   56m
```

In iptables, in the chain OUTPUT, we can observe that the `KUBE-SERVICES` chain comes before the `CNI-HOSTPORT-DNAT`, which is the chain taking care of the hostport functionality:
```
Chain OUTPUT (policy ACCEPT)
target     prot opt source               destination         
KUBE-SERVICES  all  --  0.0.0.0/0            0.0.0.0/0            /* kubernetes service portals */
CNI-HOSTPORT-DNAT  all  --  0.0.0.0/0            0.0.0.0/0            ADDRTYPE match dst-type LOCAL
```

In the KUBE-SERVICES chain, we can observe that there is one rule for each of the external-IP & port pairs, which start with `KUBE-EXT-`:
```
Chain KUBE-SERVICES (2 references)
target     prot opt source               destination         
KUBE-SVC-Z4ANX4WAEWEBLCTM  tcp  --  0.0.0.0/0            10.43.55.117         /* kube-system/metrics-server:https cluster IP */ tcp dpt:443
KUBE-SVC-UQMCRMJZLI3FTLDP  tcp  --  0.0.0.0/0            10.43.206.216        /* kube-system/traefik:web cluster IP */ tcp dpt:80
KUBE-EXT-UQMCRMJZLI3FTLDP  tcp  --  0.0.0.0/0            10.1.1.13            /* kube-system/traefik:web loadbalancer IP */ tcp dpt:80
KUBE-EXT-UQMCRMJZLI3FTLDP  tcp  --  0.0.0.0/0            10.1.1.16            /* kube-system/traefik:web loadbalancer IP */ tcp dpt:80
KUBE-SVC-CVG3OEGEH7H5P3HQ  tcp  --  0.0.0.0/0            10.43.206.216        /* kube-system/traefik:websecure cluster IP */ tcp dpt:443
KUBE-EXT-CVG3OEGEH7H5P3HQ  tcp  --  0.0.0.0/0            10.1.1.13            /* kube-system/traefik:websecure loadbalancer IP */ tcp dpt:443
KUBE-EXT-CVG3OEGEH7H5P3HQ  tcp  --  0.0.0.0/0            10.1.1.16            /* kube-system/traefik:websecure loadbalancer IP */ tcp dpt:443
KUBE-SVC-NPX46M4PTMTKRN6Y  tcp  --  0.0.0.0/0            10.43.0.1            /* default/kubernetes:https cluster IP */ tcp dpt:443
KUBE-SVC-JD5MR3NA4I4DYORP  tcp  --  0.0.0.0/0            10.43.0.10           /* kube-system/kube-dns:metrics cluster IP */ tcp dpt:9153
KUBE-SVC-TCOU7JCQXEZGVUNU  udp  --  0.0.0.0/0            10.43.0.10           /* kube-system/kube-dns:dns cluster IP */ udp dpt:53
KUBE-SVC-ERIFXISQEP7F7OF4  tcp  --  0.0.0.0/0            10.43.0.10           /* kube-system/kube-dns:dns-tcp cluster IP */ tcp dpt:53
KUBE-NODEPORTS  all  --  0.0.0.0/0            0.0.0.0/0            /* kubernetes service nodeports; NOTE: this must be the last rule in this chain */ ADDRTYPE match dst-type LOCAL
```

Those `KUBE-EXT` chains, end up calling the rule starting with `KUBE-SVC-` which replaces the destination IP with the IP of one of pods implementing the service. For example:
```
Chain KUBE-EXT-CVG3OEGEH7H5P3HQ (4 references)
target     prot opt source               destination         
KUBE-MARK-MASQ  all  --  0.0.0.0/0            0.0.0.0/0            /* masquerade traffic for kube-system/traefik:websecure external destinations */
KUBE-SVC-CVG3OEGEH7H5P3HQ  all  --  0.0.0.0/0            0.0.0.0/0   
```

As a consequence, the traffic never gets into the svclb daemonset pod. This can be additionally demonstrated by running a tcpdump on the svclb daemonset pod and no traffic will appear. This can also be demonstrated by tracing the iptables flow, where we will see how traffic is following the described path.

In Kubernetes 1.29, a [new related kube-proxy feature](https://kubernetes.io/docs/concepts/services-networking/service/#load-balancer-ip-mode) was added and it became beta in 1.30. This feature allows the load-balancer controller to specify if kube-proxy should include that `KUBE-EXT` rule or not. The selection is controlled by a new field called ipMode that has two values: * VIP (default): Keeps the current behaviour
* Proxy: Removes the `KUBE-EXT` rule and hence Kube-proxy does not react to changes in the external-ip field


Therefore, if our k3s load-balancer controller sets ipMode=Proxy, the traffic would finally get into the svclb daemon


## Solutions

One quick solution would be to use ipMode=Proxy when setting the status of the LoadBalancer services. However, it must be noted that klipper-lb is using iptables to do exactly what kube-proxy is doing, so there is no real benefit. We could include some extra features such as `X-Forwarded-For` to preserve the source IP, which would give a good reason to forward traffic to the svclb daemonset. Nonetheless, to achive that, we would need to replace klipper-lb with a proper load-balancer, which is out of the scope of this ADR. Note as well, that svclb is not working as expected when using `externalTrafficPolicy: Local`: it allows access to the service via a node IP even if there is no instance of a service pod running on that node

Another solution is to get rid of the daemonset completely. However, that solution will not detect if there is a process already using the port in a node (by another k8s service or by a node server) because kube-proxy does not check this. Unfortunately, this solution will obscure the error to the user and make debugging more difficult.

One simpler solution would be to replace klipper-lb with a tiny image that includes the binary `sleep`. Additionally, we would remove all the extra permissions required to run klipper-lb (e.g. NET_ADMIN). In this case, we would use the daemonset to reserve the port by HostPort and if a node is already using that port, that node's IP will not be included as external-IP. In such a case, the daemonset pod running on that node will clearly warn the user that it can't run because the port is already reserved, hence making it easy to debug.

This commit includes the last solution by setting busybox as the new image for the daemonset instead of klipper-lb.

## Decision

----

## Consequences

### Positives
* One fewer repo to maintain (klipper-lb)
* Easier to understand flow of traffic

### Negatives
* None that I can think of
