---
title: K3s Agent Configuration Reference
weight: 2
---
In this section, you'll learn how to configure the K3s agent.

> Throughout the K3s documentation, you will see some options that can be passed in as both command flags and environment variables. For help with passing in options, refer to [How to Use Flags and Environment Variables.]({{<baseurl>}}/k3s/latest/en/installation/install-options/how-to-flags)

- [Logging](#logging)
- [Cluster Options](#cluster-options)
- [Data](#data)
- [Node](#node)
- [Runtime](#runtime)
- [Networking](#networking)
- [Customized Flags](#customized-flags)
- [Experimental](#experimental)
- [Deprecated](#deprecated)
- [Node Labels and Taints for Agents](#node-labels-and-taints-for-agents)
- [K3s Agent CLI Help](#k3s-agent-cli-help)

### Logging

| Flag | Default | Description |
|------|---------|-------------|
|   `-v` value    |     0         | Number for the log level verbosity        |
|   `--vmodule` value   | N/A        | Comma-separated list of pattern=N settings for file-filtered logging        |
|   `--log value, -l` value  |  N/A    | Log to file   |
|   `--alsologtostderr`  | N/A        | Log to standard error as well as file (if set)     | 

### Cluster Options
| Flag | Environment Variable | Description |
|------|----------------------|-------------|
|   `--token value, -t` value  | `K3S_TOKEN`    | Token to use for authentication    |
|   `--token-file` value   |  `K3S_TOKEN_FILE`     | Token file to use for authentication       |
|   `--server value, -s` value  | `K3S_URL`    | Server to connect to     |


### Data
| Flag | Default | Description |
|------|---------|-------------|
|   `--data-dir value, -d` value  | "/var/lib/rancher/k3s"    |  Folder to hold state |

### Node
| Flag | Environment Variable | Description |
|------|----------------------|-------------|
|   `--node-name` value |  `K3S_NODE_NAME`      |  Node name       |
|   `--with-node-id`    |  N/A         | Append id to node name      |
|   `--node-label` value |    N/A        |  Registering and starting kubelet with set of labels   |
|   `--node-taint` value |      N/A     | Registering kubelet with set of taints    |

### Runtime
| Flag | Default | Description |
|------|---------|-------------|
|   `--docker` |      N/A        |      Use docker instead of containerd       |
|   `--container-runtime-endpoint` value | N/A   |  Disable embedded containerd and use alternative CRI implementation |
|   `--pause-image` value | "docker.io/rancher/pause:3.1"     |  Customized pause image for containerd or docker sandbox       | (agent/runtime)  (default: )
|   `--private-registry` value | "/etc/rancher/k3s/registries.yaml"    |   Private registry configuration file   |

### Networking
| Flag | Environment Variable | Description |
|------|----------------------|-------------|
|   `--node-ip value, -i` value | N/A   |   IP address to advertise for node  |
|   `--node-external-ip` value |  N/A   | External IP address to advertise for node      |
|   `--resolv-conf` value |   `K3S_RESOLV_CONF`    |  Kubelet resolv.conf file      | 
|   `--flannel-iface` value |    N/A   | Override default flannel interface      |
|   `--flannel-conf` value |    N/A     |  Override default flannel config file |

### Customized Flags
| Flag |  Description |
|------|--------------|
|   `--kubelet-arg` value |   Customized flag for kubelet process      | 
|   `--kube-proxy-arg` value |   Customized flag for kube-proxy process    |

### Experimental
| Flag |  Description |
|------|--------------|
|   `--rootless`  |     Run rootless           |

### Deprecated
| Flag | Environment Variable | Description |
|------|----------------------|-------------|
|   `--no-flannel`   |   N/A       |   Use `--flannel-backend=none`       | 
|   `--cluster-secret` value  |   `K3S_CLUSTER_SECRET`     |    Use `--token` |

### Node Labels and Taints for Agents

K3s agents can be configured with the options `--node-label` and `--node-taint` which adds a label and taint to the kubelet. The two options only add labels and/or taints at registration time, so they can only be added once and not changed after that again by running K3s commands.

Below is an example showing how to add labels and a taint:
```bash
     --node-label foo=bar \
     --node-label hello=world \
     --node-taint key1=value1:NoExecute
```

If you want to change node labels and taints after node registration you should use `kubectl`. Refer to the official Kubernetes documentation for details on how to add [taints](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) and [node labels.](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/#add-a-label-to-a-node)

### K3s Agent CLI Help

> If an option appears in brackets below, for example `[$K3S_URL]`, it means that the option can be passed in as an environment variable of that name.

```bash
NAME:
   k3s agent - Run node agent

USAGE:
   k3s agent [OPTIONS]

OPTIONS:
   -v value                            (logging) Number for the log level verbosity (default: 0)
   --vmodule value                     (logging) Comma-separated list of pattern=N settings for file-filtered logging
   --log value, -l value               (logging) Log to file
   --alsologtostderr                   (logging) Log to standard error as well as file (if set)
   --token value, -t value             (cluster) Token to use for authentication [$K3S_TOKEN]
   --token-file value                  (cluster) Token file to use for authentication [$K3S_TOKEN_FILE]
   --server value, -s value            (cluster) Server to connect to [$K3S_URL]
   --data-dir value, -d value          (agent/data) Folder to hold state (default: "/var/lib/rancher/k3s")
   --node-name value                   (agent/node) Node name [$K3S_NODE_NAME]
   --with-node-id                      (agent/node) Append id to node name
   --node-label value                  (agent/node) Registering and starting kubelet with set of labels
   --node-taint value                  (agent/node) Registering kubelet with set of taints
   --docker                            (agent/runtime) Use docker instead of containerd
   --container-runtime-endpoint value  (agent/runtime) Disable embedded containerd and use alternative CRI implementation
   --pause-image value                 (agent/runtime) Customized pause image for containerd or docker sandbox (default: "docker.io/rancher/pause:3.1")
   --private-registry value            (agent/runtime) Private registry configuration file (default: "/etc/rancher/k3s/registries.yaml")
   --node-ip value, -i value           (agent/networking) IP address to advertise for node
   --node-external-ip value            (agent/networking) External IP address to advertise for node
   --resolv-conf value                 (agent/networking) Kubelet resolv.conf file [$K3S_RESOLV_CONF]
   --flannel-iface value               (agent/networking) Override default flannel interface
   --flannel-conf value                (agent/networking) Override default flannel config file
   --kubelet-arg value                 (agent/flags) Customized flag for kubelet process
   --kube-proxy-arg value              (agent/flags) Customized flag for kube-proxy process
   --rootless                          (experimental) Run rootless
   --no-flannel                        (deprecated) use --flannel-backend=none
   --cluster-secret value              (deprecated) use --token [$K3S_CLUSTER_SECRET]
```
