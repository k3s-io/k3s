# Store etcd snapshot metadata in a Custom Resource

Date: 2023-07-27

## Status

Accepted

## Context

K3s currently stores a list of etcd snapshots and associated metadata in a ConfigMap. Other downstream
projects and controllers consume the content of this ConfigMap in order to present cluster administrators with
a list of snapshots that can be restored.

On clusters with more than a handful of nodes, and reasonable snapshot intervals and retention periods, the snapshot
list ConfigMap frequently reaches the maximum size allowed by Kubernetes, and fails to store any additional information.
The snapshots are still created, but they cannot be discovered by users or accessed by tools that consume information
from the ConfigMap.

When this occurs, the K3s service log shows errors such as:
```
level=error msg="failed to save local snapshot data to configmap: ConfigMap \"k3s-etcd-snapshots\" is invalid: []: Too long: must have at most 1048576 bytes"
```

A side-effect of this is that snapshot metadata is lost if the ConfigMap cannot be updated, as the list is the only place that it is stored.

Reference:
* https://github.com/rancher/rke2/issues/4495
* https://github.com/k3s-io/k3s/blob/36645e7311e9bdbbf2adb79ecd8bd68556bc86f6/pkg/etcd/etcd.go#L1503-L1516

### Existing Work

Rancher already has a `rke.cattle.io/v1 ETCDSnapshot` Custom Resource that contains the same information after it's been
imported by the management cluster:
* https://github.com/rancher/rancher/blob/027246f77f03b82660dc2e91df6bf2cd549163f0/pkg/apis/rke.cattle.io/v1/etcd.go#L48-L74

It is unlikely that we would want to use this custom resource in its current package; we may be able to negotiate moving
it into a neutral project for use by both projects.

## Decision

1. Instead of populating snapshots into a ConfigMap using the JSON serialization of the private `snapshotFile` type, K3s
   will manage creation of an new Custom Resource Definition with similar fields.
2. Metadata on each snapshot will be stored in a distinct Custom Resource.
3. The new Custom Resource will be cluster-scoped, as etcd and its snapshots are a cluster-level resource.
4. Snapshot metadata will also be written alongside snapshot files created on disk and/or uploaded to S3. The metadata
   files will have the same basename as their corresponding snapshot file.
5. A hash of the server token will be stored as an annotation on the Custom Resource, and stored as metadata on snapshots uploaded to S3.
   This hash should be compared to a current etcd snapshot's token hash to determine if the server token must be rolled back as part of the
   snapshot restore process.
6. Downstream consumers of etcd snapshot lists will migrate to watching Custom Resource types, instead of the ConfigMap.
7. K3s will observe a three minor version transition period, where both the new Custom Resources, and the existing
   ConfigMap, will both be used.
8. During the transition period, older snapshot metadata may be removed from the ConfigMap while those snapshots still
   exist and are referenced by new Custom Resources, if the ConfigMap exceeds a preset size or key count limit.

## Consequences

* Snapshot metadata will no longer be lost when the number of snapshots exceeds what can be stored in the ConfigMap.
* There will be some additional complexity in managing the new Custom Resource, and working with other projects to migrate to using it.
