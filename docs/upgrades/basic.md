---
title: "Upgrade Basics"
weight: 10
---

You can upgrade K3s by using the installation script, or by manually installing the binary of the desired version.

>**Note:** When upgrading, upgrade server nodes first one at a time, then any worker nodes.

### Release Channels

Upgrades performed via the installation script or using our [automated upgrades]({{< baseurl >}}/k3s/latest/en/upgrades/automated/) feature can be tied to different release channels. The following channels are available:

| Channel |   Description  |
|---------------|---------|
|      stable     | (Default) Stable is recommended for production environments. These releases have been through a period of community hardening. |
|      latest      | Latest is recommended for trying out the latest features.  These releases have not yet been through a period of community hardening. |
|      v1.18 (example)      | There is a release channel tied to each supported Kubernetes minor version. At the time of this writing, they are `v1.18`, `v1.17`, and `v1.16`. These channels will select the latest patch available, not necessarily a stable release. |

For an exhaustive and up-to-date list of channels, you can visit the [k3s channel service API](https://update.k3s.io/v1-release/channels). For more technical details on how channels work, you see the [channelserver project](https://github.com/rancher/channelserver).

### Upgrade K3s Using the Installation Script

To upgrade K3s from an older version you can re-run the installation script using the same flags, for example:

```sh
curl -sfL https://get.k3s.io | sh -
```
This will upgrade to a newer version in the stable channel by default.

If you want to upgrade to a newer version in a specific channel (such as latest) you can specify the channel:
```sh
curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=latest sh -
```

If you want to upgrade to a specific version you can run the following command:

```sh
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=vX.Y.Z-rc1 sh -
```

### Manually Upgrade K3s Using the Binary

Or to manually upgrade K3s:

1. Download the desired version of the K3s binary from [releases](https://github.com/rancher/k3s/releases)
2. Copy the downloaded binary to `/usr/local/bin/k3s` (or your desired location)
3. Stop the old k3s binary
4. Launch the new k3s binary

### Restarting K3s

Restarting K3s is supported by the installation script for systemd and OpenRC.

**systemd**

To restart servers manually:
```sh
sudo systemctl restart k3s
```

To restart agents manually:
```sh
sudo systemctl restart k3s-agent
```

**OpenRC**

To restart servers manually:
```sh
sudo service k3s restart
```

To restart agents manually:
```sh
sudo service k3s-agent restart
```
