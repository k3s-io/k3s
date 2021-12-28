# Testing Standards in K3s

Go testing in K3s comes in 3 forms: Unit, Integration, and End-to-End (E2E). This
document will explain *when* each test should be written and *how* each test should be
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
See the [etcd unit test](https://github.com/k3s-io/k3s/blob/master/pkg/etcd/etcd_test.go) as an example.

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

Integration tests can be placed in two areas:  

1. Next to the go package they intend to test.
2. In `tests/integration/<TESTNAME>` for package agnostic testing.  

Package specific integration tests should use the `<PACKAGE_UNDER_TEST>_test` package.  
Package agnostic integration tests should use the `integration` package.  
All integration test files should be named: `<TEST_NAME>_int_test.go`  
All integration test functions should be named: `Test_Integration<Test_Name>`.  
See the [etcd snapshot test](https://github.com/k3s-io/k3s/blob/master/pkg/etcd/etcd_int_test.go) as a package specific example.  
See the [local storage test](https://github.com/k3s-io/k3s/blob/master/tests/integration/localstorage/localstorage_int_test.go) as a package agnostic example.

### Running

Integration tests can be run with no k3s cluster present, each test will spin up and kill the appropriate k3s server it needs.  
Note: Integration tests must be run as root, prefix the commands below with `sudo -E env "PATH=$PATH"` if a sudo user.
```bash
go test ./pkg/... ./tests/integration/... -run Integration
```

Integration tests can be run on an existing single-node cluster via compile time flag, tests will skip if the server is not configured correctly.
```bash
go test -ldflags "-X 'github.com/k3s-io/k3s/tests/util.existingServer=True'" ./pkg/... ./tests/integration/... -run Integration
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

End-to-end tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the integration tests, but rely on separate testing utilities and are self-contained within the `test/e2e` directory. E2E tests cover complete K3s single and multi-cluster configuration and administration: bringup, update, teardown etc.  
E2E tests are run nightly as part of K3s quality assurance (QA).

### Format

All E2E tests should be placed under the `e2e` package.  
All E2E test functions should be named: `Test_E2E<TEST_NAME>`.  
See the [upgrade cluster test](https://github.com/k3s-io/k3s/blob/master/tests/e2e/upgradecluster_test.go) as an example.

### Running

Generally, E2E tests are run as a nightly Jenkins job for QA. They can still be run locally but additional setup may be required.

```bash
go test ./tests/e2e... -run E2E
```

## Contributing New Or Updated Tests

___
We gladly accept new and updated tests of all types. If you wish to create
a new test or update an existing test, please submit a PR with a title that includes the words `<NAME_OF_TEST> (Created/Updated)`.
