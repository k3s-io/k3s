
# How to Use Flags and Environment Variables

Throughout the K3s documentation, you will see some options that can be passed in as both command flags and environment variables. The below examples show how these options can be passed in both ways.

### Example A: K3S_KUBECONFIG_MODE

The option to allow writing to the kubeconfig file is useful for allowing a K3s cluster to be imported into Rancher. Below are two ways to pass in the option.

Using the flag `--write-kubeconfig-mode 644`:

```bash
$ curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644
```
Using the environment variable `K3S_KUBECONFIG_MODE`:

```bash
$ curl -sfL https://get.k3s.io | K3S_KUBECONFIG_MODE="644" sh -s -
```

### Example B: INSTALL_K3S_EXEC

If this command is not specified as a server or agent command, it will default to "agent" if `K3S_URL` is set, or "server" if it is not set.

The final systemd command resolves to a combination of this environment variable and script args. To illustrate this, the following commands result in the same behavior of registering a server without flannel:

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--flannel-backend none" sh -s -
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --flannel-backend none" sh -s -
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server" sh -s - --flannel-backend none
curl -sfL https://get.k3s.io | sh -s - server --flannel-backend none
curl -sfL https://get.k3s.io | sh -s - --flannel-backend none
```

### Example C: CONFIG FILE

Before installing k3s, you can create a file called `config.yaml` containing fields that match CLI flags. That file needs to be in the path: `/etc/rancher/k3s/config.yaml` for k3s to consume it.

The fields in the config file drop the starting `--` from the matching CLI flag. For example:

```
write-kubeconfig-mode: 644
token: "secret"
node-ip: 10.0.10.22,2a05:d012:c6f:4655:d73c:c825:a184:1b75 
cluster-cidr: 10.42.0.0/16,2001:cafe:42:0::/56
service-cidr: 10.43.0.0/16,2001:cafe:42:1::/112
disable-network-policy: true
```