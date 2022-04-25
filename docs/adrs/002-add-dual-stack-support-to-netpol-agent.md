# 1. Add dual stack support to netpol agent

Date: 2021-12-13

## Status

Accepted

## Context

Currently the network policy agent included in k3s is in fact a copied code of
[kube-router's](https://github.com/cloudnativelabs/kube-router) network policy
controller, which can be found [here](https://github.com/k3s-io/k3s/tree/master/pkg/agent/netpol).

The first and the most important issue is that kube-router lacks support for
dual-stack (and even IPv6 in general in the most of its components). However,
implementing such support is a non-trivial task.

The second issue is that we include a copy of kube-router code in k3s, which
makes it hard to consume updates. Even if we need some changes on top of
upstream code, we should rather use a fork which is easy to rebase with
upstream.

## Decision

We implement a feature of supporting dual stack Kubernetes clusters in network
policy controller in kube-router. We start from network policy controller, as
we don't consume any other kube-router components in k3s.

Once it's done and working, we submit a pull request:

* to [our fork of kube-router](https://github.com/k3s-io/kube-router)
* [upstream](https://github.com/cloudnativelabs/kube-router)

The motivation behind keeping a fork is that:

* upstream might ask for implementing dual-stack in all kube-router
  components (which would be understandable)
* acceepting a solution upstream might take long time

Our fork of kube-router is going to be used as a vendored library inside k3s
code. And the currently copied code in k3s in `pkg/agent/netpol` is going to
be removed in favor of using kube-router as a library.

As soon as dual-stack becomes an upstream feature - k3s is going to use
upstream kube-router as a vendored library.

## Consequences

It will increase k3s product portfolio by fully supporting dual-stack
networking.

However, it might also introduce significant amount of work for developers
which could be related to:

* agreeing with proper solution upstream
* maintaining a fork until that happens (rebasing with upstream releases)
