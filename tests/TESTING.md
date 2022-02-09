# Testing Standards in K3s

Testing in K3s comes in 4 forms: 
- [Unit](#unit-tests)
- [Integration](#integration-tests)
- [Smoke](#smoke-tests)
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

### Framework

All integration tests in K3s follow a [Behavior Diven Development (BDD)](https://en.wikipedia.org/wiki/Behavior-driven_development) style. Specifically, K3s uses [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) to drive the tests.  
To generate an initial test, the command `ginkgo bootstrap` can be used.

To facilitate K3s CLI testing, see `tests/util/cmd.go` helper functions.

### Format

All integration tests should be placed under `tests/integration/<TEST_NAME>`.  
All integration test files should be named: `<TEST_NAME>_int_test.go`.  
All integration test functions should be named: `Test_Integration<TEST_NAME>`.  
See the [local storage test](../tests/integration/localstorage/localstorage_int_test.go) as an example.

### Running

Integration tests can be run with no k3s cluster present, each test will spin up and kill the appropriate k3s server it needs.  
Note: Integration tests must be run as root, prefix the commands below with `sudo -E env "PATH=$PATH"` if a sudo user.
```bash
go test ./tests/integration/... -run Integration
```

Additionally, to generate JUnit reporting for the tests, the Ginkgo CLI is used
```
ginkgo --junit-report=result.xml ./tests/integration/...
```

Integration tests can be run on an existing single-node cluster via compile time flag, tests will skip if the server is not configured correctly.
```bash
go test -ldflags "-X 'github.com/rancher/k3s/tests/util.existingServer=True'" ./tests/integration/... -run Integration
```

Integration tests can also be run via a [Sonobuoy](https://sonobuoy.io/docs/v0.53.2/) plugin on an existing single-node cluster.
```bash
./scripts/build-tests-sonobuoy
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy run --plugin ./dist/artifacts/k3s-int-tests.yaml
```
Check the sonobuoy status and retrieve results
``` 
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy status
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy retrieve
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy results <TAR_FILE_FROM_RETRIEVE>
```

___

## Smoke Tests

Smoke tests are defined under the [tests/vagrant](../tests/vagrant) path at the root of this repository.
The sub-directories therein contain fixtures for running simple clusters to assert correct behavior for "happy path"
scenarios. These fixtures are mostly self-contained Vagrantfiles describing single-node installations that are
easily spun up with Vagrant for the `libvirt` and `virtualbox` providers:

- [Install Script](../tests/vagrant/install) :arrow_right: on proposed changes to [install.sh](../install.sh) 
  - [CentOS 7](../tests/vagrant/install/centos-7) (stand-in for RHEL 7)
  - [CentOS 8](../tests/vagrant/install/centos-8) (stand-in for RHEL 8)
  - [Leap 15.3](../tests/vagrant/install/opensuse-microos) (stand-in for SLES)
  - [MicroOS](../tests/vagrant/install/opensuse-microos) (stand-in for SLE-Micro)
  - [Ubuntu 20.04](../tests/vagrant/install/ubuntu-focal) (Focal Fossa)
- [Control Groups](../tests/vagrant/cgroup) :arrow_right: on any code change
  - [mode=unified](../tests/vagrant/cgroup/unified) (cgroups v2)
    - [Fedora 34](../tests/vagrant/cgroup/unified/fedora-34) (rootfull + rootless)
- [Snapshotter](../tests/vagrant/snapshotter/btrfs/opensuse-leap) :arrow_right: on any code change
  - [BTRFS](../tests/vagrant/snapshotter/btrfs) ([containerd built-in](https://github.com/containerd/containerd/tree/main/snapshots/btrfs))
    - [Leap 15.3](../tests/vagrant/snapshotter/btrfs/opensuse-leap)

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
cd tests/vagrant/install/centos-8
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

## End-to-End (E2E) Tests

E2E tests cover multi-node K3s configuration and administration: bringup, update, teardown etc. across a wide range of operating systems. E2E tests are run nightly as part of K3s quality assurance (QA).

### Framework 
End-to-end tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the integration tests, but rely on [Vagrant](https://www.vagrantup.com/) to provide the underlying cluster configuration. 

Currently tested operating systems are:
- [Ubuntu 20.04](https://app.vagrantup.com/generic/boxes/ubuntu2004)
- [Leap 15.3](https://app.vagrantup.com/opensuse/boxes/Leap-15.3.x86_64) (stand-in for SLE-Server)
- [MicroOS](https://app.vagrantup.com/dweomer/boxes/microos.amd64) (stand-in for SLE-Micro)

### Format

All E2E tests should be placed under `tests/e2e/<TEST_NAME>`.  
All E2E test functions should be named: `Test_E2E<TEST_NAME>`.  
A E2E test consists of two parts:
1. `Vagrantfile`: a vagrant file which describes and configures the VMs upon which the cluster and test will run
2. `<TEST_NAME>.go`: A go test file which calls `vagrant up` and controls the actual testing of the cluster

See the [validate cluster test](../tests/e2e/validatecluster/validatecluster_test.go) as an example.

### Running

Generally, E2E tests are run as a nightly Jenkins job for QA. They can still be run locally but additional setup may be required. By default, all E2E tests are designed with `libvirt` as the underlying VM provider. Instructions for installing libvirt and its associated vagrant plugin, `vagrant-libvirt` can be found [here.](https://github.com/vagrant-libvirt/vagrant-libvirt#installation) `VirtualBox` is also supported as a backup VM provider.

Once setup is complete, all E2E tests can be run with:
```bash
go test -timeout=15m ./tests/e2e/... -run E2E
```
Tests can be run individually with:
```bash
go test -timeout=15m ./tests/e2e/validatecluster/... -run E2E
#or
go test -timeout=15m ./tests/e2e/... -run E2EClusterValidation
```

Additionally, to generate junit reporting for the tests, the Ginkgo CLI is used. Installation instructions can be found [here.](https://onsi.github.io/ginkgo/#getting-started)  

To run the all E2E tests and generate JUnit testing reports:
```
ginkgo --junit-report=result.xml ./tests/e2e/...
```

Note: The `go test` default timeout is 10 minutes, thus the `-timeout` flag should be used. The `ginkgo` default timeout is 1 hour, no timeout flag is needed.

## Contributing New Or Updated Tests

___
We gladly accept new and updated tests of all types. If you wish to create
a new test or update an existing test, please submit a PR with a title that includes the words `<NAME_OF_TEST> (Created/Updated)`.
