# Secrets Encryption v3

Date: 2023-04-26

## Status

Accepted

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
1) `k3s secrets-encrypt rotate-keys`

For HA etcd:
1) `k3s secrets-encrypt rotate-keys`
2) Restart all k3s servers

### CLI Changes
The dicussion/problem is around how to go about deprecating the old method and introducing the new method. As Brad pointed out in https://github.com/k3s-io/k3s/pull/7848, we have a standard deprecation policy for CLI flags with a 2-minor release schedule, fully defined in [this ADR](./deprecating-and-removing-flags.md). Unfortunately, we need to ensure the smoothest transition with Rancher provising. Having multiple releases where some CLI is deprecated and others are not is not ideal.

One solution is to a extend support for the old process until all K3s maintained releases support the new command. Only then would the old commands be deprecated in a single cycle.

v1.28: 
- Add `k3s secrets-encrypt rotate-keys` command, marked as experimental

v1.29:
- `prepare, reencrypt, rotate` will be marked as deprecated and a warning, encouraging users to use the new `rotate-keys`.
- `rotate-keys` will go GA
- Documentation will be updated to reflect the new process, and point new installs to using the new command.

...

v1.32 or v1.33 (depending on when 1.28 goes EOL and we drop support):
- `prepare, reencrypt, new-rotate` will give fatal errors and point to the documentation on the `rotate-keys` command.
- Documentation will be updated to remove the old commands.

v1.34: 
- `prepare, reencrypt, rotate` will be removed from the codebase.

## Decision

We will continue forward with the above plan. First release with the new command will be v1.28, and the last release with the old commands will be v1.33 or v1.32, whichever minor release comes after v1.28 goes EOL.

## Consequences
This extends the number of releases we continue to support the old commands by 3-4 more than the standard deprecation process.