
# Installation Options

This page focuses on the options that can be used when you set up K3s for the first time:

- [Options for installation with script](#options-for-installation-with-script)
- [Options for installation from binary](#options-for-installation-from-binary)
- [Registration options for the K3s server](#registration-options-for-the-k3s-server)
- [Registration options for the K3s agent](#registration-options-for-the-k3s-agent)
- [Configuration File](#configuration-file)

In addition to configuring K3s with environment variables and CLI arguments, K3s can also use a [config file.](#configuration-file)

For more advanced options, refer to [this page.](../../advanced.md)

> Throughout the K3s documentation, you will see some options that can be passed in as both command flags and environment variables. For help with passing in options, refer to [How to Use Flags and Environment Variables.](how_to_flags.md)

### Options for Installation with Script

As mentioned in the [Quick-Start Guide](../../quick_start.md), you can use the installation script available at https://get.k3s.io to install K3s as a service on systemd and openrc based systems.

The simplest form of this command is as follows:
```sh
curl -sfL https://get.k3s.io | sh -
```

When using this method to install K3s, the following environment variables can be used to configure the installation:

| Environment Variable | Description |
|-----------------------------|---------------------------------------------|
| `INSTALL_K3S_SKIP_DOWNLOAD` | If set to true will not download K3s hash or binary. |
| `INSTALL_K3S_SYMLINK` | By default will create symlinks for the kubectl, crictl, and ctr binaries if the commands do not already exist in path. If set to 'skip' will not create symlinks and 'force' will overwrite. |
| `INSTALL_K3S_SKIP_ENABLE` | If set to true will not enable or start K3s service. |
| `INSTALL_K3S_SKIP_START` | If set to true will not start K3s service. |
| `INSTALL_K3S_VERSION` | Version of K3s to download from Github. Will attempt to download from the stable channel if not specified. |
| `INSTALL_K3S_BIN_DIR` | Directory to install K3s binary, links, and uninstall script to, or use `/usr/local/bin` as the default. |
| `INSTALL_K3S_BIN_DIR_READ_ONLY` | If set to true will not write files to `INSTALL_K3S_BIN_DIR`, forces setting `INSTALL_K3S_SKIP_DOWNLOAD=true`. |
| `INSTALL_K3S_SYSTEMD_DIR` | Directory to install systemd service and environment files to, or use `/etc/systemd/system` as the default. |
| `INSTALL_K3S_EXEC` | Command with flags to use for launching K3s in the service. If the command is not specified, and the `K3S_URL` is set, it will default to "agent." If `K3S_URL` not set, it will default to "server." For help, refer to [this example.](how_to_flags.md#example-b-install-k3s-exec) |
| `INSTALL_K3S_NAME` | Name of systemd service to create, will default to 'k3s' if running k3s as a server and 'k3s-agent' if running k3s as an agent. If specified the name will be prefixed with 'k3s-'. |
| `INSTALL_K3S_TYPE` | Type of systemd service to create, will default from the K3s exec command if not specified. |
| `INSTALL_K3S_SELINUX_WARN` | If set to true will continue if k3s-selinux policy is not found. |
| `INSTALL_K3S_SKIP_SELINUX_RPM` | If set to true will skip automatic installation of the k3s RPM. |
| `INSTALL_K3S_CHANNEL_URL` | Channel URL for fetching K3s download URL. Defaults to https://update.k3s.io/v1-release/channels. |
| `INSTALL_K3S_CHANNEL` | Channel to use for fetching K3s download URL. Defaults to "stable". Options include: `stable`, `latest`, `testing`. |

This example shows where to place aforementioned environment variables as options (after the pipe):

```
curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=latest sh -
```

Environment variables which begin with `K3S_` will be preserved for the systemd and openrc services to use.

Setting `K3S_URL` without explicitly setting an exec command will default the command to "agent".

When running the agent `K3S_TOKEN` must also be set.

### Options for installation from binary

As stated, the installation script is primarily concerned with configuring K3s to run as a service. If you choose to not use the script, you can run K3s simply by downloading the binary from our [release page](https://github.com/rancher/k3s/releases/latest), placing it on your path, and executing it. The K3s binary supports the following commands:

Command | Description
--------|------------------
<span class='nowrap'>`k3s server`</span> | Run the K3s management server, which will also launch Kubernetes control plane components such as the API server, controller-manager, and scheduler.
<span class='nowrap'>`k3s agent`</span> |  Run the K3s node agent. This will cause K3s to run as a worker node, launching the Kubernetes node services `kubelet` and `kube-proxy`.
<span class='nowrap'>`k3s kubectl`</span> | Run an embedded [kubectl](https://kubernetes.io/docs/reference/kubectl/overview/) CLI. If the `KUBECONFIG` environment variable is not set, this will automatically attempt to use the config file that is created at `/etc/rancher/k3s/k3s.yaml` when launching a K3s server node.
<span class='nowrap'>`k3s crictl`</span> | Run an embedded [crictl](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/crictl.md). This is a CLI for interacting with Kubernetes's container runtime interface (CRI). Useful for debugging.
<span class='nowrap'>`k3s ctr`</span> | Run an embedded [ctr](https://github.com/projectatomic/containerd/blob/master/docs/cli.md). This is a CLI for containerd, the container daemon used by K3s. Useful for debugging.
<span class='nowrap'>`k3s help`</span> | Shows a list of commands or help for one command

The `k3s server` and `k3s agent` commands have additional configuration options that can be viewed with <span class='nowrap'>`k3s server --help`</span> or <span class='nowrap'>`k3s agent --help`</span>.

### Registration Options for the K3s Server

For details on configuring the K3s server, refer to the [server configuration reference.](server_config.md)


### Registration Options for the K3s Agent

For details on configuring the K3s agent, refer to the [agent configuration reference.](agent_config.md)

### Configuration File

_Available as of v1.19.1+k3s1_

In addition to configuring K3s with environment variables and CLI arguments, K3s can also use a config file.

By default, values present in a YAML file located at `/etc/rancher/k3s/config.yaml` will be used on install.

An example of a basic `server` config file is below:

```yaml
write-kubeconfig-mode: "0644"
tls-san:
  - "foo.local"
node-label:
  - "foo=bar"
  - "something=amazing"
```

In general, CLI arguments map to their respective YAML key, with repeatable CLI arguments being represented as YAML lists.

An identical configuration using solely CLI arguments is shown below to demonstrate this:

```bash
k3s server \
  --write-kubeconfig-mode "0644"    \
  --tls-san "foo.local"             \
  --node-label "foo=bar"            \
  --node-label "something=amazing"
```

It is also possible to use both a configuration file and CLI arguments.  In these situations, values will be loaded from both sources, but CLI arguments will take precedence.  For repeatable arguments such as `--node-label`, the CLI arguments will overwrite all values in the list.

Finally, the location of the config file can be changed either through the cli argument `--config FILE, -c FILE`, or the environment variable `$K3S_CONFIG_FILE`.
