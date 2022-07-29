# Package cri-dockerd to replace dockershim

Date: 2022-07-29

## Status

Accepted

## Context

Upstream Kubernetes dropped dockershim from the kubelet in the 1.24 branch: https://kubernetes.io/blog/2022/02/17/dockershim-faq/

This means that the docker container runtime is no longer directly supported by the Kubelet; continuing to use it
requires installing cri-dockerd to translate the CRI API to the Docker API. After some internal discussion, it was
decided that we did not wish to include dockershim, in favor of requiring users to deploy cri-dockerd themselves.

```
<BD> what’s our roadmap for dockershim/cri-dockerd migration? Kubernetes 1.24 finally drops dockershim.
<BD> RKE will clearly need to migrate over to cri-dockerd, but for products like K3s where docker is supported but not the default, do we want to keep it around? Seems to work OK in K3s with some slight modifications.
<CJ> If we don't do this work, can a user configure k3s manually to use the shim?
<CJ> I'd rather reduce the surface area if possible
<BD> the work is already done, the question is do we want to include it lol
<CJ> Understood, the second half of the question is the part I care about
<BD> kk
<CJ> Can they manually get there without it
<CJ> If they can, then I don't want to include it. Less surface area for bug fixes and CVEs. And i don't see a need to make it easy for users to user docker with k3s.
<BD> users should be able to install and start cri-dockerd and then run k3s agent --container-runtime-endpoint=unix:///var/run/cri-dockerd.sock and it’ll work. 
<BD> But we would need to drop the --docker flag since that explicitly uses the kubelet’s dockershim
<BD> Well, we historically did make it easy. If we want to stop, that’s fine.
<CK> Isn’t K3s supposed to be “lightweight” anyway? Who’s to say we didn’t put it on a diet for 1.24 and made it get smaller (especially cause the containerd split made it bigger)
<CJ> Ok, I THINK bill will be ok with this. CW, can you pitch to bill next week (um he might be out too) that we are dropping the docker flag with 1.24. I guess we have a little time, so no rush right?
<CJ> with the way the ecosystem has evolved, I think we can easily justify the diet
<CJ> does k3d use/need --docker for anything?
<CJ> Ask thorsten to be sure but I didn't  think so.
<CJ> But we do need product to agree, fyi
```

The initial releases of 1.24 shipped without docker support; use of the `--docker` flag returns an error indicating that
the user should install and use cri-dockerd instead. This has been somewhat disruptive to the community; K3s is often
used in CI or dev environments where it is useful to use docker as the container runtime, but users are not equipped to
install and manage cri-dockerd due to its relative inaccessibility compared to docker itself. Ref:
https://github.com/k3s-io/k3s/issues/5741

## Decision

* We will embed cri-dockerd, and start it when the `--docker` flag is used.  This is a drop-in replacement for K3s's
  previous behavior when the Kubelet included dockershim. This meets user expectations around K3s support for the Docker
  container runtime, and eases user adoption of Kubernetes 1.24.

## Consequences

* The size of our self-extracting binary and Docker images increase by several megabytes.
* We take on the support burden of keeping cri-dockerd up to date, and supporting docker as a container runtime.
