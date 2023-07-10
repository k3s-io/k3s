# Secrets Encryption v3

Date: 2023-04-26

## Status

Proposed

## Context

### Current Secrets Encryption
We currently support rotating secrets encryption keys in the following manner:

For single server sqlite:
1) `k3s secrets-encrypt prepare`
2) Restart server
3) `k3s secrets-encrypt rotate`
4) Restart server
5) `k3s secrets-encrypt reencrypt`

For HA etcd:
1) `k3s secrets-encrypt prepare`
2) Restart all k3s servers
3) `k3s secrets-encrypt rotate`
4) Restart all k3s servers
5) `k3s secrets-encrypt reencrypt`
6) Restart all k3s servers

This is a lot of manual restarts and downtime. 

### New Upstream Feature
With the introduction of [Automatic Config Reloading](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#configure-automatic-reloading), a component of [KMV v2](https://github.com/kubernetes/enhancements/issues/3299) currently in Beta as of v1.27, with a GA target of v1.29, we can reduce this to:

For single server sqlite:
1) `k3s secrets-encrypt rotate`

For HA etcd:
1) `k3s secrets-encrypt rotate`
2) Restart all k3s servers

### CLI Changes
The dicussion/problem is around how to go about deprecating the old method and introducing the new method. As Brad pointed out in https://github.com/k3s-io/k3s/pull/7848, we have a standard deprecation policy for CLI flags with a 2-minor release schedule, fully defined in [this ADR](./deprecating-and-removing-flags.md). However, ultimately it would be best if the new CLI was a replacement, not a new command. 

One possible solution is to a add a "transitional", expirmental, command for a single release, with explicit documentation that this is only for the transition period. As an example:

v1.27 and older: 
- Warnings that the old methods `prepare, rotate, reencrypt` will be deprecated in v1.28

v1.28: 
- Add `k3s secrets-encrypt new-rotate` command, with warnings that this is only for avaliable in v1.28, is expiremental, and will become `k3s secrets-encrypt rotate` in v1.29. 
- CLI calls to `prepare, reencrypt` will print a warning that they are deprecated and will be removed in v1.29
- CLI call to `rotate` will print a warning that it is deprecated and will be replaced with `new-rotate` in v1.29

v1.29:
- CLI calls to `prepare, reencrypt` will fatal error and point user to `rotate` with the new v3 documentation.
- CLI calls to `rotate` will now be the new v3 method, with no warnings.
- CLI calls to `new-rotate` will give a warning it is deprecated, that `rotate` should now be used, and will be removed in v1.30

v1.30:
- CLI calls for `prepare, reencrypt, new-rotate` will be removed from the codebase.

## Decision

## Consequences
This cuts short the number of releases we will handle `new-rotate` by 1 compared to the full approach. By being upfront about its expiremental and temporary nature, we can hopefully avoid any confusion or issues with users. It also places in line with when upstream KMS v2 will go GA.