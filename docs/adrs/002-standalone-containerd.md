# 2. Move containerd out of the k3s multicall bundle

Date: 2021-12-15

## Status

Accepted

## Context

In the process up updated K3s to Kubernetes 1.23, we encountered the following problems:
* Kubernetes 1.23 now requires cri-api runtime v1. It will gracefully fall back if the remote server only implements v1alpha2 services, but the go module itself must be v1.
* Containerd v1.5 is not compatible with cri-api runtime v1; it doesn't implement all the required services.
* Containerd v1.6 DOES implement cri-api runtime v1, but it's still in beta - and also uses opentelemetry v1.0
* Kubernetes and etcd 2.5 are still on opentelemetry v0.20, which cannot be used alongside v1.0 - almost all of the modules were reorganized during the move to v1.0

Optimistically, this will be resolved by Kubernetes 1.24 - containerd 1.6 will be out of beta, and Kubernetes and etcd will upgrade to opentelemetry 1.0. At that point we can go back to embedding containerd in the main k3s process. Until then (for probably all of the 1.23 branch) we will need to build containerd separately.
* https://github.com/containerd/containerd/pull/6113#issuecomment-984226639
* https://github.com/kubernetes/kubernetes/issues/106536

There is another issue with go-genproto needing to be held back for compatibility with ttrpc. The version we're currently pinning isn't compatible with cel-go, which Kubernetes now uses for server-side CRD field validation. Hopefully that will be resolved in containerd v1.6 as well.
* https://github.com/containerd/ttrpc/issues/62#issuecomment-903075627

## Decision

* We will build containerd as a standalone binary, instead of being part of the k3s multicall bundle.
* Once upstream library conflicts are resolved, we will attempt to move containerd back into the multicall bundle.

## Consequences

The size of our self-extracting binary and Docker images increase by several megabytes.
