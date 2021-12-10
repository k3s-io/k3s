---
title: The k3s-killall.sh script
weight: 4
---

To allow high availability during upgrades, the K3s containers continue running when the K3s service is stopped.

To stop all of the K3s containers and reset the containerd state, the `k3s-killall.sh` script can be used.

The killall script cleans up containers, K3s directories, and networking components while also removing the iptables chain with all the associated rules. The cluster data will not be deleted.

To run the killall script from a server node, run:

```
/usr/local/bin/k3s-killall.sh
```