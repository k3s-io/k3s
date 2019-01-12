# oci-runtime-tool [![Build Status](https://travis-ci.org/opencontainers/runtime-tools.svg?branch=master)](https://travis-ci.org/opencontainers/runtime-tools) [![Go Report Card](https://goreportcard.com/badge/github.com/opencontainers/runtime-tools)](https://goreportcard.com/report/github.com/opencontainers/runtime-tools)

oci-runtime-tool is a collection of tools for working with the [OCI runtime specification][runtime-spec].
To build from source code, runtime-tools requires Go 1.7.x or above.

## Generating an OCI runtime spec configuration files

[`oci-runtime-tool generate`][generate.1] generates [configuration JSON][config.json] for an [OCI bundle][bundle].
[OCI-compatible runtimes][runtime-spec] like [runC][] expect to read the configuration from `config.json`.

```console
$ oci-runtime-tool generate --output config.json
$ cat config.json
{
        "ociVersion": "0.5.0",
        …
}
```

## Validating an OCI bundle

[`oci-runtime-tool validate`][validate.1] validates an OCI bundle.
The error message will be printed if the OCI bundle failed the validation procedure.

```console
$ oci-runtime-tool generate
$ oci-runtime-tool validate
INFO[0000] Bundle validation succeeded.
```

## Testing OCI runtimes

The runtime validation suite uses [node-tap][], which is packaged for some distributions (for example, it is in [Debian's `node-tap` package][debian-node-tap]).
If your distribution does not package node-tap, you can install [npm][] (for example, from [Gentoo's `nodejs` package][gentoo-nodejs]) and use it:

```console
$ npm install tap
```

Build the validation executables:

```console
$ make runtimetest validation-executables
```

Runtime validation currently [only supports](docs/runtime-compliance-testing.md) the [OCI Runtime Command Line Interface](doc/command-line-interface.md).
If we add support for alternative APIs in the future, runtime validation will gain an option to select the desired runtime API.
For the command line interface, the `RUNTIME` option selects the runtime command (`funC` in the [OCI Runtime Command Line Interface](doc/command-line-interface.md)).

```
$ sudo make RUNTIME=runc localvalidation
RUNTIME=runc tap validation/pidfile.t validation/linux_cgroups_hugetlb.t validation/linux_cgroups_memory.t validation/linux_rootfs_propagation_shared.t validation/kill.t validation/create.t validation/poststart.t validation/linux_cgroups_network.t validation/poststop_fail.t validation/linux_readonly_paths.t validation/prestart_fail.t validation/hooks_stdin.t validation/default.t validation/linux_masked_paths.t validation/poststop.t validation/misc_props.t validation/prestart.t validation/poststart_fail.t validation/mounts.t validation/linux_cgroups_relative_pids.t validation/process_user.t validation/process.t validation/hooks.t validation/process_capabilities_fail.t validation/process_rlimits_fail.t validation/linux_cgroups_relative_cpus.t validation/process_rlimits.t validation/linux_cgroups_relative_blkio.t validation/linux_sysctl.t validation/linux_seccomp.t validation/linux_devices.t validation/start.t validation/linux_cgroups_pids.t validation/process_capabilities.t validation/process_oom_score_adj.t validation/linux_cgroups_relative_hugetlb.t validation/linux_cgroups_cpus.t validation/linux_cgroups_relative_memory.t validation/state.t validation/root_readonly_true.t validation/linux_cgroups_blkio.t validation/linux_rootfs_propagation_unbindable.t validation/delete.t validation/linux_cgroups_relative_network.t validation/hostname.t validation/killsig.t validation/linux_uid_mappings.t
validation/pidfile.t .failed to create the container
container_linux.go:348: starting container process caused "process_linux.go:402: container init caused \"process_linux.go:367: setting cgroup config for procHooks process caused \\\"failed to write 56892210544640 to hugetlb.1GB.limit_in_bytes: open /sys/fs/cgroup/hugetlb/cgrouptest/hugetlb.1GB.limit_in_bytes: permission denied\\\"\""
exit status 1
validation/pidfile.t .................................. 1/1 315ms
validation/linux_cgroups_hugetlb.t .................... 0/1
  not ok validation/linux_cgroups_hugetlb.t
    timeout: 30000
    file: validation/linux_cgroups_hugetlb.t
    command: validation/linux_cgroups_hugetlb.t
    args: []
    stdio:
      - 0
      - pipe
      - 2
    cwd: /…/go/src/github.com/opencontainers/runtime-tools
    exitCode: 1

validation/linux_cgroups_memory.t ..................... 9/9
validation/linux_rootfs_propagation_shared.t ...... 252/282
  not ok shared root propogation exposes "/target348456609/mount892511628/example376408222"

  Skipped: 29
     /dev/null (default device) has unconfigured permissions
…
total ........................................... 4381/4962


  4381 passing (1m)
  567 pending
  14 failing

make: *** [Makefile:44: localvalidation] Error 1
```

You can also run an individual test executable directly:

```console
$ RUNTIME=runc validation/default.t
TAP version 13
ok 1 - has expected hostname
  ---
  {
    "actual": "mrsdalloway",
    "expected": "mrsdalloway"
  }
  ...
…
ok 287 # SKIP linux.gidMappings not set
1..287
```

If you cannot install node-tap, you can probably run the test suite with another [TAP consumer][tap-consumers].
For example, with [`prove`][prove]:

```console
$ sudo make TAP='prove -Q -j9' RUNTIME=runc VALIDATION_TESTS=validation/pidfile.t localvalidation
RUNTIME=runc prove -Q -j9 validation/pidfile.t
All tests successful.
Files=1, Tests=1,  0 wallclock secs ( 0.01 usr  0.01 sys +  0.03 cusr  0.03 csys =  0.08 CPU)
Result: PASS
```

[bundle]: https://github.com/opencontainers/runtime-spec/blob/master/bundle.md
[config.json]: https://github.com/opencontainers/runtime-spec/blob/master/config.md
[debian-node-tap]: https://packages.debian.org/stretch/node-tap
[debian-nodejs]: https://packages.debian.org/stretch/nodejs
[gentoo-nodejs]: https://packages.gentoo.org/packages/net-libs/nodejs
[node-tap]: http://www.node-tap.org/
[npm]: https://www.npmjs.com/
[prove]: http://search.cpan.org/~leont/Test-Harness-3.39/bin/prove
[runC]: https://github.com/opencontainers/runc
[runtime-spec]: https://github.com/opencontainers/runtime-spec
[tap-consumers]: https://testanything.org/consumers.html

[generate.1]: man/oci-runtime-tool-generate.1.md
[validate.1]: man/oci-runtime-tool-validate.1.md
