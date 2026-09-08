# Pull K3s System Images from GHCR

Date: 2026-09-08

## Status

Proposed

## Context

K3s is losing its exemption from Docker Hub rate limiting, and every image K3s deploys is pulled
from Docker Hub today. See [k3s-io/k3s#14561](https://github.com/k3s-io/k3s/issues/14561).

Users are not the only ones exposed. CI is a heavy anonymous puller: the airgap job pulls the full
list once per architecture, and a single pass brings up clusters across nine e2e scenarios, sixteen
Docker suites over two architectures, and six install distributions. GitHub-hosted runners share a
pool of IP addresses, so the quota is not even ours alone, and pre-release runs multiply the job
count exactly when a failed pull is most expensive. A throttled pull surfaces as `ImagePullBackOff`
and a test timeout rather than as a quota error, so it gets chased as a flake.

The eight images in `scripts/airgap/image-list.txt` are referenced without a registry host, so
`--system-default-registry` was applied by prefixing, and an empty setting left containerd to
resolve against `docker.io`. Docker Hub was never named as our default anywhere in the tree; we
inherited it from the runtime's convention.

Those images do not all belong to the same org, which decides what we can do with each:

* `klipper-lb` and `klipper-helm` are built by K3s, and are already published to `ghcr.io/k3s-io`
  with exact platform parity to their Docker Hub counterparts.
* `local-path-provisioner` is built by Rancher. As of
  [rancher/local-path-provisioner#627](https://github.com/rancher/local-path-provisioner/pull/627),
  its release workflow publishes to `ghcr.io/rancher` alongside Docker Hub, but the newest release,
  `v0.0.37`, predates that change. `ghcr.io/rancher/local-path-provisioner` therefore carries a
  single `master-head` tag today, and the first release to land there will be `v0.0.38`.
* The five `mirrored-*` images belong to upstream projects: BusyBox and Traefik are Docker Hub
  official images, CoreDNS is published by the CoreDNS project, and metrics-server and pause come
  from `registry.k8s.io`. What K3s pulls today are Rancher's `rancher/mirrored-*` copies of them,
  kept for RKE2 and Rancher. K3s mirrors those projects from upstream directly and publishes the
  result to `ghcr.io/k3s-io`; it does not re-host Rancher's copies. The `mirrored-` prefix stays
  only so the repository names remain recognizable.

### Why GHCR

K3s already publishes there, the `k3s-io` org exists, and Actions authenticates with the
`GITHUB_TOKEN` it already issues, so no new credentials are introduced. Public packages are not
billed for storage or egress, and their pulls are not metered against the per-IP quota Docker Hub
applies to anonymous clients - which covers both sides above, the user bringing up a cluster and
the CI matrix during a release cycle.

#### Alternatives considered

* **Move only the images K3s builds, and leave the `mirrored-*` ones on Docker Hub.** The smaller
  change, and it avoids republishing images K3s does not build. It also leaves most of the exposure
  standing: six of the eight images, including the pause image every pod on the node depends on,
  would still be pulled from the registry we are moving away from. It does nothing for CI either,
  where one throttled pull fails the job regardless of which image it was.
* **quay.io.** A second hosting relationship to negotiate and a new credential in CI, for a registry
  that is not already part of the release workflow. It has its own anonymous pull quota, so the
  problem is traded rather than solved.
* **registry.k8s.io.** Serves artifacts released by Kubernetes SIGs. A distribution's service load
  balancer and Helm job runner are not Kubernetes project artifacts, and publishing there would need
  acceptance by the release engineering group that runs it.

## Decision

* Every image in `scripts/airgap/image-list.txt` is pulled from `ghcr.io`, named explicitly in the
  reference rather than left to the runtime's default. A default install stops reaching Docker Hub.
* The images K3s already builds keep the org that builds them: `ghcr.io/k3s-io/klipper-lb` and
  `ghcr.io/k3s-io/klipper-helm`.
* K3s mirrors the five `mirrored-*` images to `ghcr.io/k3s-io`, taking each one from its upstream
  publisher rather than from Rancher's copy.
* `local-path-provisioner` is mirrored alongside them, as
  `ghcr.io/k3s-io/mirrored-local-path-provisioner`, rather than taken from `ghcr.io/rancher`.
  Rancher has not cut `v0.0.38` yet, so the only options there are a Docker Hub release or a
  floating `master-head`. Mirroring `v0.0.37` ourselves keeps the manifest pinned to a release
  while staying off Docker Hub. Once `v0.0.38` is published to `ghcr.io/rancher`, this image can
  point back at the org that builds it.
* `ghcr.io` does **not** become a global default. It is the default for the images K3s deploys, and
  for nothing else. `config.Control.SystemDefaultRegistry` stays empty unless the operator sets it,
  so containerd keeps resolving against its own default and a pod that asks for `library/busybox`
  still gets it from Docker Hub.
* `--system-default-registry` remains the only supported way to serve system images from elsewhere,
  and now covers both reference styles: one that names a registry has that host replaced, carrying
  its path and tag along; one that does not, such as a user-supplied `--pause-image`, is still
  prefixed.
* The images are still published to Docker Hub under `rancher/<image>`. GHCR is primary, not
  exclusive.
* The release workflow repoints the published image list by replacing each entry's leading registry
  host, rather than substituting the literal string `docker.io`.

#### How the default is applied

The images are declared in two places, so the default reaches them two ways. Both resolve to the
same host, and neither writes into `config.Control.SystemDefaultRegistry`, which is what keeps the
containerd default for user workloads out of it.

Manifests name the host through a `%{SYSTEM_IMAGE_REGISTRY}%` template variable that `stageFiles`
expands, so the YAML keeps a complete image reference that updatecli can still match and bump:

```yaml
image: "%{SYSTEM_IMAGE_REGISTRY}%/k3s-io/mirrored-coredns-coredns:1.14.7"
```

The variable is deliberately not `%{SYSTEM_DEFAULT_REGISTRY}%`, which stays as it is for manifests
users drop into `server/manifests`: that one expands to nothing when unset, and ours never does.
`traefik.yaml` is the exception, because the Traefik chart builds its own reference out of
`global.systemDefaultRegistry`, `image.repository` and `image.tag`. It takes the same variable as a
Helm value and the chart supplies the separator. The older `%{SYSTEM_DEFAULT_REGISTRY_RAW}%` existed
only to serve that spot with a copy that had no trailing slash; a variable that is never empty does
not need to carry its own separator, so the two uses collapse into one.

## Consequences

* Availability and rate limits for the images K3s deploys stop being inherited from Docker Hub's
  commercial terms. Because every image moves rather than most of them, the exposure is removed for
  a default install and for a CI run, instead of being reduced to whichever image is left.
* Replacing the host rather than prefixing it changes the result for an operator who already pairs
  `--system-default-registry` with a fully qualified override such as `--pause-image`. That used to
  yield `<registry>/docker.io/rancher/mirrored-pause`, it now yields `<registry>/rancher/mirrored-pause`. 
  The old result reads like a defect, but anyone depending on it loses it quietly, since the new reference is valid and simply resolves to nothing.
* Airgap tarballs built before this change carry the old references. `imageTagNames` retags imported
  images using `docker.Path`, so an old tarball retagged into a private registry yields
  `<registry>/rancher/klipper-lb` while K3s now asks for `<registry>/k3s-io/klipper-lb`. Airgap
  users must take a matching tarball and K3s version.
* Version bump automation now verifies the GHCR copy rather than the Docker Hub one, so updatecli
  will not propose a version that is not on the registry K3s pulls from.
