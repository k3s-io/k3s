# Publish System Images to GHCR Under the k3s-io Organization

Date: 2026-09-08

## Status

Proposed

## Context

K3s is losing its exemption from Docker Hub rate limiting, and every image K3s deploys is pulled
from Docker Hub today. See k3s-io/k3s#14561.

The eight images listed in `scripts/airgap/image-list.txt` were referenced without a registry host -
`rancher/klipper-lb:v0.4.17`, and so on. That was deliberate: `--system-default-registry` was applied
by prefixing the configured registry onto the reference, in `pkg/server/server.go`,
`pkg/daemons/control/deps/deps.go`, `pkg/agent/config/config.go`, and `pkg/cli/server/server.go`.
When the setting was empty no prefix was applied, and the reference resolved against containerd's
implicit default of `docker.io`. Docker Hub was therefore never named as our default anywhere in the
tree; we inherited it from the runtime's convention, and it could not be changed without changing
every reference.

Beyond the rate limit, six of the eight images live in an organization K3s does not control.
`rancher/mirrored-coredns-coredns`, `mirrored-library-busybox`, `mirrored-library-traefik`,
`mirrored-metrics-server`, `mirrored-pause`, and `local-path-provisioner` are published by Rancher
and shared with RKE2 and Rancher. Their retention, availability, and publishing cadence are decided
elsewhere. Only `klipper-lb` and `klipper-helm` are published to `ghcr.io/k3s-io` today, both with
exact platform parity to their Docker Hub counterparts.

## Decision

* K3s will publish all of its system images to `ghcr.io` under the `k3s-io` organization, mirroring
  the six images it does not currently own rather than depending on another organization's copies.
* `ghcr.io` becomes the default registry for K3s system images. `--system-default-registry` remains
  the single supported way to pull them from anywhere else.
* The images continue to be published to Docker Hub as well. GHCR is the primary, not the only,
  location. **The Docker Hub copy will still be published as the same name the image is in GHCR** 
  - will still use `rancher/<image>` for docker. `--system-default-registry` replaces the registry
  host and carries the repository path along unchanged.
<!-- * Image references become fully qualified. `util.DefaultRegistry` names the default, and
  `util.ImageWithRegistry` applies `--system-default-registry` by *replacing* the registry host
  rather than prefixing the reference. References that do not name a registry, such as a
  user-supplied `--pause-image`, are still prefixed, so operator-facing behavior is unchanged. -->
* `%{SYSTEM_DEFAULT_REGISTRY}%` and `%{SYSTEM_DEFAULT_REGISTRY_RAW}%` expand to `ghcr.io` when no
  registry is configured, instead of expanding to nothing. Manifests therefore name a registry
  explicitly rather than deferring to the runtime's default.
* The release workflow repoints the published image list by replacing the leading registry host of
  each entry, rather than substituting the literal string `docker.io`.

## Consequences

* K3s controls the registry its images are served from. Availability, retention, and rate limits for
  system images become a K3s decision rather than an inherited one.
* Airgap tarballs built before this change contain the old references. `imageTagNames` retags
  imported images using `docker.Path`, which returns everything after the domain, so an old tarball
  retagged into a private registry yields `<registry>/rancher/...` while K3s now asks for
  `<registry>/k3s-io/...`. Airgap users must take a matching tarball and K3s version.
* We take on the ongoing cost of running the mirror: keeping it in step with upstream, covering every
  platform K3s builds for, and retaining tags for as long as the supported releases that reference
  them.