containerd is built with OCI support and with support for advanced features
provided by [runc](https://github.com/opencontainers/runc).

Development (`-dev`) and pre-releases of containerd may depend features in `runc`
that have not yet been released, and may require a specific runc build. The version
of runc that is tested against in our CI can be found in the [`script/setup/runc-version`](../script/setup/runc-version)
file, which may point to a git-commit (for pre releases) or tag in the runc
repository.

For regular (non-pre-)releases of containerd releases, we attempt to use released
(tagged) versions of runc. We recommend using a version of runc that's equal to
or higher than the version of runc described in [`script/setup/runc-version`](../script/setup/runc-version).

If you encounter any runtime errors, make sure your runc is in sync with the
commit or tag provided in that file.

## building

> For more information on how to clone and build runc also refer to the runc
> building [documentation](https://github.com/opencontainers/runc#building).

Before building runc you may need to install additional build dependencies, which
will vary by platform. For example, you may need to install `libseccomp` e.g.
`libseccomp-dev` for Ubuntu.

From within your `opencontainers/runc` repository run:

```bash
make && sudo make install
```

Starting with runc 1.0.0-rc93, the "selinux" and "apparmor" buildtags have been
removed, and runc builds have SELinux, AppArmor, and seccomp support enabled
by default. Note that "seccomp" can be disabled by passing an empty `BUILDTAGS`
make variable, but is highly recommended to keep enabled.

By default, runc is compiled with kernel-memory limiting support enabled. This
functionality is deprecated in kernel 5.4 and up, and is known to be broken on
RHEL7 and CentOS 7 3.10 kernels. For these kernels, we recommend disabling kmem
support using the `nokmem` build-tag. When doing so, be sure to set the `seccomp`
build-tag to enable seccomp support, for example:

```sh
make BUILDTAGS='nokmem seccomp' && make install
```

For details about the `nokmem` build-tag, refer to the discussion on [opencontainers/runc#2594](https://github.com/opencontainers/runc/pull/2594).
For further details on building runc, refer to the [build instructions in the runc README](https://github.com/opencontainers/runc#building).
