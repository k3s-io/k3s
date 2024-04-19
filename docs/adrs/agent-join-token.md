# Support `kubeadm`-style Bootstrap Token Secrets

Date: 2022-12-20

## Status

Accepted

## Context

### K3s Token Types and Use

K3s currently supports two tokens that can be used to join nodes to the cluster:
* `--token`: This is the default token, and a random value is generated during initial cluster startup if not
  specified by the user. This token is also used as the passphrase input to the PBKDF2 function used to generate
  the encryption key for cluster bootstrap data. For this reason, all server nodes must use the same token value
  once the cluster has been started, and the token value cannot be changed.
* `--agent-token`: By default, this is set to the same as the `--token` value. If set, this token can be used
  to join new agents to the cluster, but not servers. This token value can be changed after the cluster has
  beens started, but doing so requires coordinatating reconfiguration and restart of all of the servers in the
  cluster.

Internally, these tokens are used as the password for HTTP Basic authentication to the K3s supervisor when the
agent bootstraps its configuration and certificates. Servers use a username of `server`, while agents
(including servers local agents) use `node`. Once nodes join the cluster they also populate a node password
secret that prevents other nodes from using the same node name, but this is unrelated to the token.

### Security Considerations

Users have requested the ability to generate single-use or limited-duration tokens that can be used to join
nodes to the cluster, but can be deleted or automatically expire in order to reduce the impact should the
token be compromised. Currently, compromise of the server token would require a complete rebuild of the
cluster in order to use a new token. Compromise of the agent token would require a coordinated restart of all
nodes in the cluster.

### Existing Work

`kubeadm` includes a `kubeadm token create` command that creates secrets of type
`bootstrap.kubernetes.io/token`, which is a core upstream type that is not restricted for use by kubeadm.

There are helpers for interacting with bootstrap token secrets in the `k8s.io/cluster-bootstrap` package, and
upstream Kubernetes includes two controllers (`tokencleaner` and `bootstrapsigner`) to support use of cluster
bootstrap secrets. The latter controller is not relevant for our use case, as it serves the same function as
existing K3s supervisor routes - making initial cluster CA certificates and a client kubeconfig available for
bootstrapping nodes. The [boostrap-tokens](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/)
documentation can be referenced for more information.

## Decision

* K3s will allow joining agents to the cluster using bootstrap token secrets.
* K3s will NOT allow joining servers to the cluster using bootstrap token secrets.
* K3s will include a `k3s token` subcommand that allows for token create/list/delete operations, similar to
  the functionality offered by `kubeadm`.
* K3s will enable the `tokencleaner` controller, in order to ensure that bootstrap token secrets are cleaned
  up when their TTL expires.
* K3s agent bootstrap functionality will allow a agent to connect the cluster using existing [Node
  Authorization](https://kubernetes.io/docs/reference/access-authn-authz/node/) to authenticate to the
  cluster during startup, even after its join token has been invalidated.
* K3s agent bootstrap functionality will NOT allow an agent to connect to the cluster if it does not have a valid
  token, and its Node object has been deleted from the cluster.

## Consequences

This will require additional documentation, CLI subcommands, and QA work to validate use of bootstrap token secret auth.
