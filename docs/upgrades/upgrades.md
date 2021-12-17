---
title: "Upgrades"
weight: 25
---

This section describes how to upgrade your K3s cluster.

[Upgrade basics](basic.md) describes several techniques for upgrading your cluster manually. It can also be used as a basis for upgrading through third-party Infrastructure-as-Code tools like [Terraform](https://www.terraform.io/).

[Automated upgrades](automated.md) describes how to perform Kubernetes-native automated upgrades using Rancher's [system-upgrade-controller](https://github.com/rancher/system-upgrade-controller).

> If Traefik is not disabled K3s versions 1.20 and earlier will have installed Traefik v1, while K3s versions 1.21 and later will install Traefik v2 if v1 is not already present.  To upgrade Traefik, please refer to the [Traefik documentation](https://doc.traefik.io/traefik/migration/v1-to-v2/) and use the [migration tool](https://github.com/traefik/traefik-migration-tool) to migrate from the older Traefik v1 to Traefik v2.

> The experimental embedded Dqlite data store was deprecated in K3s v1.19.1. Please note that upgrades from experimental Dqlite to experimental embedded etcd are not supported. If you attempt an upgrade it will not succeed and data will be lost.
