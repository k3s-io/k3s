
# Air-Gap Install

You can install K3s in an air-gapped environment using two different methods. An air-gapped environment is any environment that is not directly connected to the Internet. You can either deploy a private registry and mirror docker.io, or you can manually deploy images such as for small clusters.

## Private Registry Method

This document assumes you have already created your nodes in your air-gap environment and have a Docker private registry on your bastion host.

If you have not yet set up a private Docker registry, refer to the official documentation [here](https://docs.docker.com/registry/deploying/#run-an-externally-accessible-registry).

### Create the Registry YAML

Follow the [Private Registry Configuration](private_registry.md) guide to create and configure the registry.yaml file.

Once you have completed this, you may now go to the [Install K3s](#install-k3s) section below.


## Manually Deploy Images Method

We are assuming you have created your nodes in your air-gap environment.
This method requires you to manually deploy the necessary images to each node and is appropriate for edge deployments where running a private registry is not practical.

### Prepare the Images Directory and K3s Binary
Obtain the images tar file for your architecture from the [releases](https://github.com/rancher/k3s/releases) page for the version of K3s you will be running.

Place the tar file in the `images` directory, for example:

```sh
sudo mkdir -p /var/lib/rancher/k3s/agent/images/
sudo cp ./k3s-airgap-images-$ARCH.tar /var/lib/rancher/k3s/agent/images/
```

Place the k3s binary at `/usr/local/bin/k3s` and ensure it is executable.

Follow the steps in the next section to install K3s.

## Install K3s

### Prerequisites

- Before installing K3s, complete the the [Private Registry Method](#private-registry-method) or the [Manually Deploy Images Method](#manually-deploy-images-method) above to prepopulate the images that K3s needs to install.
- Download the K3s binary from the [releases](https://github.com/rancher/k3s/releases) page, matching the same version used to get the airgap images. Place the binary in `/usr/local/bin` on each air-gapped node and ensure it is executable.
- Download the K3s install script at https://get.k3s.io. Place the install script anywhere on each air-gapped node, and name it `install.sh`.

When running the K3s script with the `INSTALL_K3S_SKIP_DOWNLOAD` environment variable, K3s will use the local version of the script and binary.


### Installing K3s in an Air-Gapped Environment

You can install K3s on one or more servers as described below.

=== "Single Server Configuration"

    To install K3s on a single server, simply do the following on the server node:

    ```
    INSTALL_K3S_SKIP_DOWNLOAD=true ./install.sh
    ```

    Then, to optionally add additional agents do the following on each agent node. Take care to ensure you replace `myserver` with the IP or valid DNS of the server and replace `mynodetoken` with the node token from the server typically at `/var/lib/rancher/k3s/server/node-token`

    ```
    INSTALL_K3S_SKIP_DOWNLOAD=true K3S_URL=https://myserver:6443 K3S_TOKEN=mynodetoken ./install.sh
    ```

=== "High Availability Configuration"

    Reference the [High Availability with an External DB](ha_external.md) or [High Availability with Embedded DB](ha_embedded.md) guides. You will be tweaking install commands so you specify `INSTALL_K3S_SKIP_DOWNLOAD=true` and run your install script locally instead of via curl. You will also utilize `INSTALL_K3S_EXEC='args'` to supply any arguments to k3s.

    For example, step two of the High Availability with an External DB guide mentions the following:

    ```
    curl -sfL https://get.k3s.io | sh -s - server \
    --datastore-endpoint='mysql://username:password@tcp(hostname:3306)/database-name'
    ```

    Instead, you would modify such examples like below:

    ```
    INSTALL_K3S_SKIP_DOWNLOAD=true INSTALL_K3S_EXEC='server' K3S_DATASTORE_ENDPOINT='mysql://username:password@tcp(hostname:3306)/database-name' ./install.sh
    ```

>**Note:** K3s additionally provides a `--resolv-conf` flag for kubelets, which may help with configuring DNS in air-gap networks.

## Upgrading

### Install Script Method

Upgrading an air-gap environment can be accomplished in the following manner:

1. Download the new air-gap images (tar file) from the [releases](https://github.com/rancher/k3s/releases) page for the version of K3s you will be upgrading to. Place the tar in the `/var/lib/rancher/k3s/agent/images/` directory on each
node. Delete the old tar file.
2. Copy and replace the old K3s binary in `/usr/local/bin` on each node. Copy over the install script at https://get.k3s.io (as it is possible it has changed since the last release). Run the script again just as you had done in the past
with the same environment variables.
3. Restart the K3s service (if not restarted automatically by installer).


### Automated Upgrades Method

As of v1.17.4+k3s1 K3s supports [automated upgrades](../upgrades/automated.md). To enable this in air-gapped environments, you must ensure the required images are available in your private registry.

You will need the version of rancher/k3s-upgrade that corresponds to the version of K3s you intend to upgrade to. Note, the image tag replaces the `+` in the K3s release with a `-` because Docker images do not support `+`.

You will also need the versions of system-upgrade-controller and kubectl that are specified in the system-upgrade-controller manifest YAML that you will deploy. Check for the latest release of the system-upgrade-controller [here](https://github.com/rancher/system-upgrade-controller/releases/latest) and download the system-upgrade-controller.yaml to determine the versions you need to push to your private registry. For example, in release v0.4.0 of the system-upgrade-controller, these images are specified in the manifest YAML:

```
rancher/system-upgrade-controller:v0.4.0
rancher/kubectl:v0.17.0
```

Once you have added the necessary rancher/k3s-upgrade, rancher/system-upgrade-controller, and rancher/kubectl images to your private registry, follow the [automated upgrades](../upgrades/automated.md) guide.
