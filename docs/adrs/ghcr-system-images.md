# Pull K3s System Images from GHCR

Date: 2026-09-08

## Status

Accepted

## Context

K3s currently uses images from the `rancher` Organization on Docker Hub, which is exempt from image
pull rate limits due to a paid contract in place between SUSE and Docker Hub. This contract is coming
to an end, and image pulls will be affected by rate limits. For more information, see
[k3s-io/k3s#14561](https://github.com/k3s-io/k3s/issues/14561).

CI is affected as well. The airgap job pulls the full image list once per architecture, and a single
pass brings up clusters across nine e2e scenarios, sixteen Docker suites over two architectures, and
six install distributions.

The eight images in `scripts/airgap/image-list.txt` are referenced without a registry host.
`--system-default-registry` is applied by prefixing that host onto the reference, and when the
setting is empty containerd resolves the reference against its own default of `docker.io`. Docker
Hub is not named as a default anywhere in the tree; it is inherited from the runtime's convention.

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
* **registry.k8s.io.** Serves artifacts released by Kubernetes SIGs. The klipper-lb and klipper-helm images
  are specific to K3s and RKE2, and it is unlikely that the upstream project would accept a request to
  publish these images to the k8s.io registry unless the whole project was moved into kubernetes-sigs,
  which is even less likely.

## Decision

* Every image in `scripts/airgap/image-list.txt` is pulled from `ghcr.io`. A default install stops
  reaching Docker Hub.
* `--system-default-registry` defaults to `ghcr.io` rather than being empty. No new mechanism is
  introduced: every bundled image reference is without host by default, and the setting is already applied
  to all of them, so changing the default is the best implementation.
* `--system-default-registry` only sets the registry for images used by bundled components. It does
  not change the implicit default registry that the runtime uses when a reference does not name one.
  Example: a pod that asks for `library/busybox` still gets it from Docker Hub.
* `docker.io/k3s-io` does not exists, so an empty `--system-default-registry` is rejected at server startup.
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
* The images are still published to Docker Hub under `rancher/<image>`. GHCR is primary, not
  exclusive.

#### How the default is applied

Manifests keep the existing `%{SYSTEM_DEFAULT_REGISTRY}%` template variable, which `stageFiles`
expands to the setting followed by a separator, so the YAML keeps a complete image reference that
updatecli can still match and bump:

```yaml
image: "%{SYSTEM_DEFAULT_REGISTRY}%k3s-io/mirrored-coredns-coredns:1.14.7"
```

## Consequences

* Availability and rate limits for the images K3s deploys stop being inherited from Docker Hub's
  commercial terms. Because every image moves rather than most of them, the exposure is removed for
  a default install and for a CI run, instead of being reduced to whichever image is left.
* An empty `--system-default-registry` used to be the normal case and now fails at startup. An
  operator carrying `system-default-registry: ""` in `config.yaml`, or automation that always passes
  the flag, has to drop it.
* The bundled references move from `rancher/<image>` to `k3s-io/<image>`. An operator serving system
  images from their own registry through `--system-default-registry` has to populate the new paths.
* Airgap tarballs built before this change carry the old references. `imageTagNames` retags imported
  images using `docker.Path`, so an old tarball retagged into a private registry yields
  `<registry>/rancher/klipper-lb` while K3s now asks for `<registry>/k3s-io/klipper-lb`. Airgap
  users must take a matching tarball and K3s version.
* Version bump automation now verifies the GHCR copy rather than the Docker Hub one, so updatecli
  will not propose a version that is not on the registry K3s pulls from.
* The migration was originally planned for v1.40, but the contract ends before that, so it lands in
  the releases that follow this ADR instead. Docker Hub keeps receiving the images under
  `rancher/<image>` through v1.40, so anyone still pulling from there has until then to repoint.
* We will also do a blogpost about this migration, and not just the release notes, since anyone
  that mirror the images will need to change them in the registry.
