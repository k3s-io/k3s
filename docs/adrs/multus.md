# Record architecture decisions

Date: 2024-04-15

## Status

Denied

## Context

### Multus

Multus is a CNI multiplexer that allows pods to have multiple network interfaces. We have users that are operating K3s + Multus but it is not super obvious how to configure it to work with K3s and how to add the additional pieces needed (e.g. IPAM or additional CNI plugins). We could facilitate this by creating an integration with Multus.

We we will wait a bit to include whereabouts. That project is using very old dependencies which will creep in CVEs

### Design suggestion

Add multus to the k3s-charts repo. That multus chart will consume the tarball we generate in rke2-charts, i.e. both rke2 and k3s will use the same chart with minimal diffs (e.g. the Chart name will be k3s-multus instead of rke2-multus).

Then, multus will be consumed as traefik:
* The chart gets downloaded with `make download`
* The chart tarball gets embedded in k3s binary with `go generate` and included in `pkg/static/zz_generated_bindata.go`
* The HelmChart manifest pointing to the chart tarball gets embedded in k3s binary with `go generate` and included in `pkg/deploy/zz_generated_bindata.go`

K3s will include a new `--multus` boolean flag. When that flag is true, we would leave the HelmChart manifest installing multus.

The multus chart will install a daemonset that:
* deploys the necessary binaries (multus and common CNI plugins) in each node
* generates the correct CNI plugin
* Installs the required CRDs

It sucks a bit that the daemonset stays dormant forever after doing the job instead of just dying, but the alternatives are worse

## Alternatives

* K3s creates a job that picks the multus and whereabouts CNI plugins from the `image-build-cni-plugins` and copies them to each node. However, configuring jobs to run on each node is not that easy and very error prone. Therefore, we decided to reject this idea

* K3s includes the multus and whereabouts CNI plugins as part of its multi-exec cni binary. However, the whereabouts binary is using very old dependencies which would creep in CVEs. Moreover, the size of the K3s binary would increase more than 10%, something not acceptable for a something that the vast majority of K3s users will not enable

### Limitations

The multus and cni-plugins images do not support ARM architecture. At this first release, that architecture is not supported

### Airgap

We are creating a different tarball that includes the multus images:
* docker.io/rancher/hardened-multus-cni
* docker.io/rancher/hardened-cni-plugins
* docker.io/rancher/mirrored-library-busybox

## Decision

The decision was to dismiss this idea because it was really complicated. The main problem is that because we install Multus using a helm chart, we need to specify the directory where the CNI binaries are (data-dir) and it must be the same for all nodes. This is complicated because this path depends on the K3s build, and so different K3s versions will surely have different paths. As a consequence, we could only use this feature in homogeneous clusters. We could make the CNIs not depend on that (data-dir), as we do when not deploying Flannel but that changes typical K3s deployments and might create extra problems. As this PR was looking to reduce users' problems, creating different problems does not really help.
