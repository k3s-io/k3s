# 4. Deprecating and Removing Flags

Date: 2022-07-20

## Status

Accepted

## Context

Currently we have several flags that are no longer [used](https://k3s-io.github.io/docs/reference/server-config#deprecated-options). These have been labelled as deprecated, but we currently lack a system around removing them completely. This creates a problem where we aren't really deprecating flags, because they are always kept around.

## Decision

The following system will be implemented for depcrecating and removing flags:

1) Flags can be declared as deprecated at any time.
2) Flags that are deprecated must be labeled as such on all currently supported releases. 
3) Once a flag has been marked as deprecated, it will be converted to a hidden flag on the next patch release. E.g. `--foo` will be marked as deprecated in v1.24.2, and will be converted to a hidden flag in v1.24.3.
   Additionally, the flag will begin to warn users that it is going to be removed in the next minor release. E.g. `--foo` will cause a warning in v1.24.3, telling users that "--foo will be removed in v1.25.0".
4) In the next minor release branch, flags deprecated will become "nonoperational" in that they will cause a fatal error if used. This error must exlapin to the user any new flags or configuration that replace this flag. 
5) In the following minor release, the nonoperational flags will be removed.

An example of the proposed system:
- `--foo` exists in v1.23.10 and v1.24.2.
- After the v1.24.2 release, it is decided to deprecate `--foo` in favor of `--new-foo`.
- In v1.23.11 and v1.24.2, `--foo` is marked as deprecated and is converted to hidden. The flag contines to exist, but will warn users "--foo will be removed in v1.25.0, convert to using `--new-foo`". `--foo` will continue to exist as a hidden and operational flag for the life of v1.23 and v1.24.
- In v1.25.0, `--foo` is marked as nonoperational and will cause a fatal error if used. The error message will say `--foo is no longer supported, use --new-foo`.
- In v1.26.0, `--foo` is removed.

## Consequences

This enables us to completely remove depreciated flags. This long process has minimal risk as the timelines for deprecation and removal are well-defined and explained to the user.  
One downside is that it will take quite a while (2 minor releases) to completely remove flags.
