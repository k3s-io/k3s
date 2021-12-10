---
title: Uninstalling K3s
weight: 61
---

If you installed K3s using the installation script, a script to uninstall K3s was generated during installation.

> Uninstalling K3s deletes the cluster data and all of the scripts. To restart the cluster with different installation options, re-run the installation script with different flags.

To uninstall K3s from a server node, run:

```
/usr/local/bin/k3s-uninstall.sh
```

To uninstall K3s from an agent node, run:

```
/usr/local/bin/k3s-agent-uninstall.sh
```