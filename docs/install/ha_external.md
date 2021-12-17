# High Availability with an External DB

> **Note:** Official support for installing Rancher on a Kubernetes cluster was introduced in our v1.0.0 release.

This section describes how to install a high-availability K3s cluster with an external database.

Single server clusters can meet a variety of use cases, but for environments where uptime of the Kubernetes control plane is critical, you can run K3s in an HA configuration. An HA K3s cluster is comprised of:

* Two or more **server nodes** that will serve the Kubernetes API and run other control plane services
* Zero or more **agent nodes** that are designated to run your apps and services
* An **external datastore** (as opposed to the embedded SQLite datastore used in single-server setups)
* A **fixed registration address** that is placed in front of the server nodes to allow agent nodes to register with the cluster

For more details on how these components work together, refer to the [architecture section.](../architecture.md#high-availability-with-an-external-db)

Agents register through the fixed registration address, but after registration they establish a connection directly to one of the server nodes. This is a websocket connection initiated by the `k3s agent` process and it is maintained by a client-side load balancer running as part of the agent process.

## Installation Outline

Setting up an HA cluster requires the following steps:

1. [Create an external datastore](#1-create-an-external-datastore)
2. [Launch server nodes](#2-launch-server-nodes)
3. [Configure the fixed registration address](#3-configure-the-fixed-registration-address)
4. [Join agent nodes](#4-optional-join-agent-nodes)

### 1. Create an External Datastore
You will first need to create an external datastore for the cluster. See the [Cluster Datastore Options](datastore.md) documentation for more details.

### 2. Launch Server Nodes
K3s requires two or more server nodes for this HA configuration. See the [Installation Requirements](install-requirements/install_requirements.md) guide for minimum machine requirements.

When running the `k3s server` command on these nodes, you must set the `datastore-endpoint` parameter so that K3s knows how to connect to the external datastore. The `token` parameter can also be used to set a deterministic token when adding nodes. When empty, this token will be generated automatically for further use.

For example, a command like the following could be used to install the K3s server with a MySQL database as the external datastore and [set a token](install-options/server_config.md#cluster-options}}):

```bash
curl -sfL https://get.k3s.io | sh -s - server \
  --token=SECRET \
  --datastore-endpoint="mysql://username:password@tcp(hostname:3306)/database-name"
```

The datastore endpoint format differs based on the database type. For details, refer to the section on [datastore endpoint formats.](datastore.md#datastore-endpoint-format-and-functionality)

To configure TLS certificates when launching server nodes, refer to the [datastore configuration guide.](datastore.md#external-datastore-configuration-parameters)

> **Note:** The same installation options available to single-server installs are also available for high-availability installs. For more details, see the [Installation and Configuration Options](install-options/install_options.md) documentation.

By default, server nodes will be schedulable and thus your workloads can get launched on them. If you wish to have a dedicated control plane where no user workloads will run, you can use taints. The <span style='white-space: nowrap'>`node-taint`</span> parameter will allow you to configure nodes with taints, for example <span style='white-space: nowrap'>`--node-taint CriticalAddonsOnly=true:NoExecute`</span>.

Once you've launched the `k3s server` process on all server nodes, ensure that the cluster has come up properly with `k3s kubectl get nodes`. You should see your server nodes in the Ready state.

### 3. Configure the Fixed Registration Address

Agent nodes need a URL to register against. This can be the IP or hostname of any of the server nodes, but in many cases those may change over time. For example, if you are running your cluster in a cloud that supports scaling groups, you may scale the server node group up and down over time, causing nodes to be created and destroyed and thus having different IPs from the initial set of server nodes. Therefore, you should have a stable endpoint in front of the server nodes that will not change over time. This endpoint can be set up using any number approaches, such as:

* A layer-4 (TCP) load balancer
* Round-robin DNS
* Virtual or elastic IP addresses

This endpoint can also be used for accessing the Kubernetes API. So you can, for example, modify your [kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file to point to it instead of a specific node. To avoid certificate errors in such a configuration, you should install the server with the `--tls-san YOUR_IP_OR_HOSTNAME_HERE` option. This option adds an additional hostname or IP as a Subject Alternative Name in the TLS cert, and it can be specified multiple times if you would like to access via both the IP and the hostname.

### 4. Optional: Join Additional Server Nodes

The same example command in Step 2 can be used to join additional server nodes, where the token from the first node needs to be used.

If the first server node was started without the `--token` CLI flag or `K3S_TOKEN` variable, the token value can be retrieved from any server already joined to the cluster:
```bash
cat /var/lib/rancher/k3s/server/token
```

Additional server nodes can then be added [using the token](install-options/server_config.md#cluster-options}}):

```bash
curl -sfL https://get.k3s.io | sh -s - server \
  --token=SECRET \
  --datastore-endpoint="mysql://username:password@tcp(hostname:3306)/database-name"
```


There are a few config flags that must be the same in all server nodes:

* Network related flags: `--cluster-dns`, `--cluster-domain`, `--cluster-cidr`, `--service-cidr`
* Flags controlling the deployment of certain components: `--disable-helm-controller`, `--disable-kube-proxy`, `--disable-network-policy` and any component passed to `--disable`
* Feature related flags: `--secrets-encryption`

> **Note:** Ensure that you retain a copy of this token as it is required when restoring from backup and adding nodes. Previously, K3s did not enforce the use of a token when using external SQL datastores.

### 5. Optional: Join Agent Nodes

Because K3s server nodes are schedulable by default, the minimum number of nodes for an HA K3s server cluster is two server nodes and zero agent nodes. To add nodes designated to run your apps and services, join agent nodes to your cluster.

Joining agent nodes in an HA cluster is the same as joining agent nodes in a single server cluster. You just need to specify the URL the agent should register to and the token it should use.

```bash
K3S_TOKEN=SECRET k3s agent --server https://fixed-registration-address:6443
```
