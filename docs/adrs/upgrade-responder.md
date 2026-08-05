# upgrade-responder client

Date: 2025-09-10

## Status

Proposed

## Context

### Background

K3s (and by extension any distribution based on it) currently lacks a mechanism
to voluntarily share version and cluster metadata. This telemetry data would be
very valuable for understanding adoption and planning future development
priorities.

Other existing CNCF projects have already long adopted (or are in the process
thereof) the upgrade-responder pattern (such as Longhorn) (see
https://github.com/longhorn/upgrade-responder).

That service provides endpoints that accept version and metadata information,
allowing maintainers to understand their user base better while respecting
privacy.

The core client side implementation is a straight-forward periodic REST API
call.

### Current State

- No telemetry collection exists in k3s
- Users have no visibility into available updates relevant to their deployment
- Product teams lack insights into deployment patterns and version adoption

### Requirements

- Collect only non-personally identifiable cluster metadata
- Opt-out mechanism with clear documentation
- Minimal resource overhead
- Fails gracefully in disconnected environments
- There is no need for retry mechanisms or a persistent daemon; the data is
  non-critical and loss of a few data points harmless. Resource savings
  on the nodes are more important.
- Work well in k3s and k3s-based distributions

## Decision

Implement an `upgrade-responder` client at
`github.com/k3s-io/k3s-upgrade-responder` (similar to existing components) as a
separate, optional component deployed via the k3s manifest system that is
triggered periodically.

### Architecture

- **Deployment Method**: `CronJob` in `kube-system` namespace
- **Location**: `/var/lib/rancher/k3s/server/manifests/upgrade-responder.yaml`
- **Scheduling**: CronJob running thrice daily (`0 */8 * * *`)
- **Configuration**: ConfigMap-based with environment variable override
- **Default State**: Enabled by default (opt-out well documented)

### Data Collection

Example payload structure:
```json
{
  "appVersion": "v1.31.6+k3s1",
  "extraTagInfo": {
    "kubernetesVersion": "v1.31.6",
    "clusteruuid": "53741f60-f208-48fc-ae81-8a969510a598"
  },
  "extraFieldInfo": {
    "nodeCount": 5,
    "serverNodeCount": 3,
    "agentNodeCount": 2
  }
}
```

The `clusteruuid` is needed to differentiate between different deployments (the
UUID of `kube-system`). It is completely random and does not expose privacy
considerations.

### Configuration Interface Example

```yaml
# /etc/rancher/k3s/config.yaml
upgrade-responder-enabled: true
upgrade-responder-config:
  endpoint: "https://k3s-upgrade-responder.rancher.io/v1/checkupgrade"
  schedule: "0 */8 * * *"
```

(The last two would be defaults if `enabled: true` but not specified.)

## Alternatives Considered

### Built-in k3s Binary Integration

The manifest-based approach provides better separation of
concerns.

### Agent-based Implementation

Would require agents on all nodes. Periodic CronJob is more efficient for
cluster-level metadata collection.

### Instrumenting/leveraging update.k3s.io

No easy access to CDN logs, no insights into deployed versions, not as
privacy-preserving.

## Consequences

Basic telemetry coverage and analytics to improve project decisions and project
visibility.

## Future options

This can also form the basis for pro-actively informing users about relevant
available updates based on their existing deployed version. This is explicitly
excluded from this ADR, as it will require additional considerations.

This could be extended with other fields (such as the kernel or Linux
distribution version). It may make sense to then not just support fully
disabling it, but to simply skip the extra fields only.
