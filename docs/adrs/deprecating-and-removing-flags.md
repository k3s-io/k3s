# Deprecating and Removing Flags

Date: 2022-07-20

## Status

Accepted

## Context

Upstream Kubernetes has a [flag deprecation policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/#deprecating-a-flag-or-cli). For admin-facing components, flags must function for at 1 minor release after their deprecation before they are removed. 

Historically, the policy around removing flags was to maintain flag compatibility for two minor releases before making any breaking changes. 
For example, flag would be:
- Supported in v1.17
- Marked deprecated in v1.18
- Warning but continues to work in v1.19
- Removed in v1.20

This policy was not well documented and was tribal knowledge.

Currently, we have several flags that are no longer [used](https://k3s-io.github.io/docs/reference/server-config#deprecated-options). These have been labelled as deprecated. However, we have failed to maintain the historical policy and have not removed the flags, even after multiple minor releases. This creates a problem where we aren't really deprecating flags, because they are always kept around.

## Decision

The following system will be implemented for deprecating and removing flags:

1) Flags can be declared as "To Be Deprecated" at any time.
2) Flags that are "To Be Deprecated" must be labeled as such on the next patch of all currently supported releases. Additionally, the flag will begin to warn users that it is going to be deprecated in the next "new" minor release. 
3) On the next minor release, a flag will be marked as deprecated in the documentation and converted to a hidden flag in code.
4) In the following minor release branch, deprecated flags will become "nonoperational" in that they will cause a fatal error if used. This error must explain to the user any new flags or configuration that replace this flag.
5) In the following minor release, the nonoperational flags will be removed from documentation and code.

An example of the proposed system:
- `--foo` exists in v1.23.10 and v1.24.2.
- After the v1.24.2 release, it is decided to deprecate `--foo` in favor of `--new-foo`.
- In v1.23.11 and v1.24.2, `--foo` continues to exist, but will warn users "--foo will be deprecated in v1.25.0, convert to using `--new-foo`". `--foo` will continue to exist as an operational flag for the life of v1.23 and v1.24.
- In v1.25.0, `--foo` is marked as deprecated in documentation and will be hidden in code. It will continue to work and warn users.
- In v1.26.0, `--foo` will cause a fatal error if used. The error message will say `--foo is no longer supported, use --new-foo`.
- In v1.27.0, `--foo` will be removed.

## Consequences

This enables us to completely remove depreciated flags. This long process has minimal risk as the timelines for deprecation and removal are well-defined and explained to the user.  
One downside is that it will take quite a while (2 minor releases) to completely remove flags.
