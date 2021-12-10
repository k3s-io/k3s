---
title: FAQ
weight: 60
---

The FAQ is updated periodically and designed to answer the questions our users most frequently ask about K3s.

**Is K3s a suitable replacement for k8s?**

K3s is capable of nearly everything k8s can do. It is just a more lightweight version. See the [main]({{<baseurl>}}/k3s/latest/en/) docs page for more details.

**How can I use my own Ingress instead of Traefik?**

Simply start K3s server with `--disable traefik` and deploy your ingress.

**Does K3s support Windows?**

At this time K3s does not natively support Windows, however we are open to the idea in the future.

**How can I build from source?**

Please reference the K3s [BUILDING.md](https://github.com/rancher/k3s/blob/master/BUILDING.md) with instructions.

**Where are the K3s logs?**

The installation script will auto-detect if your OS is using systemd or openrc and start the service.

When running with openrc, logs will be created at `/var/log/k3s.log`.

When running with systemd, logs will be created in `/var/log/syslog` and viewed using `journalctl -u k3s`.