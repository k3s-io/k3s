# Remove svclb daemonset

Date: 2024-09-26

## Status

Not approved

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

Therefore, if we replace the logic to find the node IPs of the serviceLB controller by something which does not require the svclb daemonset, we could get rid of that daemonset since traffic is never reaching it. That replacement should be easy because in the end a daemonset means all nodes, so we could basically query kube-api to provide the IPs of all nodes.


## Decision

There is one use case where klipper-lb is used. When deploying in a public cloud and using the publicIP as the --node-external-ip, kube-proxy is expecting the publicIP to be the destination IP. However, public clouds are normally doing a DNAT, so the kube-proxy's rule will never be used because the incoming packet does not have the publicIP anymore. In that case, the packet is capable of reaching the service because of the hostPort functionality on the daemonset svclb pushing the packet to svclb and then, klipper-lb routing the packet to the service. Conclusion: klipper-lb is needed 

## Consequences

### Positives
* Less resource consumption as we won't need one daemonset per LoadBalancer type of service
* One fewer repo to maintain (klipper-lb)
* Easier to understand flow of traffic

### Negatives
* Possible confusion for users that have been using this feature for a long time ("Where is the daemonset?") or users relying on that daemonset for their automation
* In today's solution, if two LoadBalancer type services are using the same port, it is rather easy to notice that things don't work because the second daemonset will not deploy, as the port is already being used by the first daemonset. Kube-proxy does not check if two services are using the same port and it will create both rules without any error. The service that gets its rules higher in the chain is the one that will be reached when querying $nodeIP:$port. Perhaps we could add some logic in the controller that warns users about a duplication of the pair ip&port
