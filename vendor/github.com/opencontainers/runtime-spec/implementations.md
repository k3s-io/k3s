# <a name="implementations" />Implementations

The following sections link to associated projects, some of which are maintained by the OCI and some of which are maintained by external organizations.
If you know of any associated projects that are not listed here, please file a pull request adding a link to that project.

## <a name="implementationsRuntimeContainer" />Runtime (Container)

* [opencontainers/runc][runc] - Reference implementation of OCI runtime
* [projectatomic/bwrap-oci][bwrap-oci] - Convert the OCI spec file to a command line for [bubblewrap][bubblewrap]
* [giuseppe/crun][crun] - Runtime implementation in C

## <a name="implementationsRuntimeVirtualMachine" />Runtime (Virtual Machine)

* [hyperhq/runv][runv] - Hypervisor-based runtime for OCI
* [clearcontainers/runtime][cc-runtime] - Hypervisor-based OCI runtime utilising [virtcontainers][virtcontainers] by Intel®.
* [google/gvisor][gvisor] - gVisor is a user-space kernel, contains runsc to run sandboxed containers.
* [kata-containers/runtime][kata-runtime] - Hypervisor-based OCI runtime combining technology from [clearcontainers/runtime][cc-runtime] and [hyperhq/runv][runv].

## <a name="implementationsTestingTools" />Testing & Tools

* [kunalkushwaha/octool][octool] - A config linter and validator.
* [huawei-openlab/oct][oct] - Open Container Testing framework for OCI configuration and runtime
* [opencontainers/runtime-tools][runtime-tools] - A config generator and runtime/bundle testing framework.


[runc]: https://github.com/opencontainers/runc
[runv]: https://github.com/hyperhq/runv
[cc-runtime]: https://github.com/clearcontainers/runtime
[kata-runtime]: https://github.com/kata-containers/runtime
[virtcontainers]: https://github.com/containers/virtcontainers
[octool]: https://github.com/kunalkushwaha/octool
[oct]: https://github.com/huawei-openlab/oct
[runtime-tools]: https://github.com/opencontainers/runtime-tools
[bwrap-oci]: https://github.com/projectatomic/bwrap-oci
[bubblewrap]: https://github.com/projectatomic/bubblewrap
[crun]: https://github.com/giuseppe/crun
[gvisor]: https://github.com/google/gvisor
