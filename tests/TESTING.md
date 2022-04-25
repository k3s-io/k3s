# Testing Standards in K3s

Testing in K3s comes in 5 forms: 
- [Unit](#unit-tests)
- [Integration](#integration-tests)
- [Smoke](#smoke-tests)
- [Performance](#performance)
- [End-to-End (E2E)](#end-to-end-e2e-tests)

This document will explain *when* each test should be written and *how* each test should be
generated, formatted, and run.

Note: all shell commands given are relative to the root k3s repo directory.
___

## Unit Tests

Unit tests should be written when a component or function of a package needs testing.
Unit tests should be used for "white box" testing.

### Framework

All unit tests in K3s follow a [Table Driven Test](https://github.com/golang/go/wiki/TableDrivenTests) style. Specifically, K3s unit tests are automatically generated using the [gotests](https://github.com/cweill/gotests) tool. This is built into the Go vscode extension, has documented integrations for other popular editors, or can be run via command line. Additionally, a set of custom templates are provided to extend the generated test's functionality. To use these templates, call:

```bash
gotests --template_dir=<PATH_TO_K3S>/contrib/gotests_templates
```

Or in vscode, edit the Go extension setting `Go: Generate Tests Flags`  
and add `--template_dir=<PATH_TO_K3S>/contrib/gotests_templates` as an item.

To facilitate unit test creation, see `tests/util/runtime.go` helper functions.

### Format

All unit tests should be placed within the package of the file they test.  
All unit test files should be named: `<FILE_UNDER_TEST>_test.go`.  
All unit test functions should be named: `Test_Unit<FUNCTION_TO_TEST>` or `Test_Unit<RECEIVER>_<METHOD_TO_TEST>`.  
See the [etcd unit test](../pkg/etcd/etcd_test.go) as an example.

### Running

```bash
go test ./pkg/... -run Unit
```

Note: As unit tests call functions directly, they are the primary drivers of K3s's code coverage
metric.

___

## Integration Tests

Integration tests should be used to test a specific functionality of k3s that exists across multiple Go packages, either via exported function calls, or more often, CLI comands.
Integration tests should be used for "black box" testing. 

See [integration/README.md](./integration/README.md) for more info.

___

## Smoke Tests

Smoke tests are a collection of tests defined under the [tests](../tests) path at the root of this repository.
The sub-directories therein contain fixtures for running simple clusters to assert correct behavior for "happy path" scenarios. These fixtures are mostly self-contained Vagrantfiles describing single-node installations that are easily spun up with Vagrant for the `libvirt` and `virtualbox` providers:

- [Install Script](../tests/install) :arrow_right: on proposed changes to [install.sh](../install.sh) 
  - [CentOS 7](../tests/install/centos-7) (stand-in for RHEL 7)
  - [Rocky Linux 8](../tests/install/rocky-8) (stand-in for RHEL 8)
  - [Leap 15.3](../tests/install/opensuse-microos) (stand-in for SLES)
  - [MicroOS](../tests/install/opensuse-microos) (stand-in for SLE-Micro)
  - [Ubuntu 20.04](../tests/install/ubuntu-focal) (Focal Fossa)
- [Control Groups](../tests/cgroup) :arrow_right: on any code change
  - [mode=unified](../tests/cgroup/unified) (cgroups v2)
    - [Fedora 34](../tests/cgroup/unified/fedora-34) (rootfull + rootless)
- [Snapshotter](../tests/snapshotter/btrfs/opensuse-leap) :arrow_right: on any code change
  - [BTRFS](../tests/snapshotter/btrfs) ([containerd built-in](https://github.com/containerd/containerd/tree/main/snapshots/btrfs))
    - [Leap 15.3](../tests/snapshotter/btrfs/opensuse-leap)

When adding new installer test(s) please copy the prevalent style for the `Vagrantfile`.
Ideally, the boxes used for additional assertions will support the default `virtualbox` provider which
enables them to be used by our Github Actions Workflow(s). See:
- [cgroup.yaml](../.github/workflows/cgroup.yaml).
- [install.yaml](../.github/workflows/install.yaml).

### Framework

If you are new to Vagrant, Hashicorp has written some pretty decent introductory tutorials and docs, see:
- https://learn.hashicorp.com/collections/vagrant/getting-started
- https://www.vagrantup.com/docs/installation

#### Plugins and Providers

The `libvirt` and `vmware_desktop` providers cannot be used without first [installing the relevant plugins](https://www.vagrantup.com/docs/cli/plugin#plugin-install)
which are [`vagrant-libvirt`](https://github.com/vagrant-libvirt/vagrant-libvirt) and
[`vagrant-vmware-desktop`](https://www.vagrantup.com/docs/providers/vmware/installation), respectively.
Much like the default [`virtualbox` provider](https://www.vagrantup.com/docs/providers/virtualbox) these will do
nothing useful without also installing the relevant server runtimes and/or client programs.

#### Environment Variables

These can be set on the CLI or exported before invoking Vagrant:
- `TEST_VM_CPUS` (default :arrow_right: 2)<br/>
  The number of vCPU for the guest to use.
- `TEST_VM_MEMORY` (default :arrow_right: 2048)<br/>
  The number of megabytes of memory for the guest to use.
- `TEST_VM_BOOT_TIMEOUT` (default :arrow_right: 600)<br/>
  The time in seconds that Vagrant will wait for the machine to boot and be accessible.

### Running

The **Install Script** tests can be run by changing to the fixture directory and invoking `vagrant up`, e.g.:
```shell
cd tests/install/rocky-8
vagrant up
# the following provisioners are optional. the do not run by default but are invoked
# explicitly by github actions workflow to avoid certain timeout issues on slow runners
vagrant provision --provision-with=k3s-wait-for-node
vagrant provision --provision-with=k3s-wait-for-coredns
vagrant provision --provision-with=k3s-wait-for-local-storage
vagrant provision --provision-with=k3s-wait-for-metrics-server
vagrant provision --provision-with=k3s-wait-for-traefik
vagrant provision --provision-with=k3s-status
vagrant provision --provision-with=k3s-procps
```

The **Control Groups** and **Snapshotter** tests require that k3s binary is built at `dist/artifacts/k3s`.
They are invoked similarly, i.e. `vagrant up`, but with different sets of named shell provisioners.
Take a look at the individual Vagrantfiles and/or the Github Actions workflows that harness them to get
an idea of how they can be invoked.

___

## Performance Tests

Performance tests use Terraform to test large scale deployments of K3s clusters.

See [perf/README.md](./perf/README.md) for more info.
___

## End-to-End (E2E) Tests

E2E tests cover multi-node K3s configuration and administration: bringup, update, teardown etc. across a wide range of operating systems. E2E tests are run nightly as part of K3s quality assurance (QA).

See [e2e/README.md](./e2e/README.md) for more info.

___

## Contributing New Or Updated Tests

We gladly accept new and updated tests of all types. If you wish to create
a new test or update an existing test, please submit a PR with a title that includes the words `<NAME_OF_TEST> (Created/Updated)`.
