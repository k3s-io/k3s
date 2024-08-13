# Testing Standards in K3s

Testing in K3s comes in 5 forms: 
- [Unit](#unit-tests)
- [Integration](#integration-tests)
- [Docker](#docker-tests)
- [Smoke](#smoke-tests)
- [Performance](#performance)
- [End-to-End (E2E)](#end-to-end-e2e-tests)
- [Distros-test-framework](#distros-test-framework)

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

Integration tests should be used to test a specific functionality of k3s that exists across multiple Go packages, either via exported function calls, or more often, CLI commands.
Integration tests should be used for "black box" testing. 

See [integration/README.md](./integration/README.md) for more info.

___

## Docker Tests

Docker tests run clusters of K3s nodes as containers and test basic functionality. These tests are run in the Drone CI pipeline `test` stage.

___

## Install Tests

Install tests are a collection of tests defined under the [tests/install](./tests/install). These tests are used to validate the installation and operation of K3s on a variety of operating systems. The test themselves are Vagrantfiles describing single-node installations that are easily spun up with Vagrant for the `libvirt` and `virtualbox` providers:

- [Install Script](install) :arrow_right: scheduled nightly and on an install script change
  - [CentOS 9 Stream](install/centos-stream)
  - [Rocky Linux 8](install/rocky-8) (stand-in for RHEL 8)
  - [Rocky Linux 9](install/rocky-9) (stand-in for RHEL 9)
  - [Fedora 40](install/fedora)
  - [Leap 15.6](install/opensuse-leap) (stand-in for SLES)
  - [Ubuntu 24.04](install/ubuntu-2404)

## Format
When adding new installer test(s) please copy the prevalent style for the `Vagrantfile`.
Ideally, the boxes used for additional assertions will support the default `libvirt` provider which
enables them to be used by our GitHub Actions [Install Test Workflow](../.github/workflows/install.yaml).

### Framework

If you are new to Vagrant, Hashicorp has written some pretty decent introductory tutorials and docs, see:
- https://learn.hashicorp.com/collections/vagrant/getting-started
- https://www.vagrantup.com/docs/installation

#### Plugins and Providers

The `libvirt`provider cannot be used without first [installing the `vagrant-libvirt` plugin](https://github.com/vagrant-libvirt/vagrant-libvirt). Libvirtd service must be installed and running on the host machine as well.

Additionally, the `vagrant-scp` and `vagrant-k3s` plugins are required.

All three can be and can be installed with:
```shell
vagrant plugin install vagrant-scp vagrant-k3s vagrant-libvirt
```

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
# The following provisioners are optional. In GitHub Actions CI they are invoked
# explicitly to avoid certain timeout issues on slow runners
vagrant provision --provision-with=k3s-wait-for-node
vagrant provision --provision-with=k3s-wait-for-coredns
vagrant provision --provision-with=k3s-wait-for-local-storage
vagrant provision --provision-with=k3s-wait-for-metrics-server
vagrant provision --provision-with=k3s-wait-for-traefik
vagrant provision --provision-with=k3s-status
vagrant provision --provision-with=k3s-procps
```

___

## Performance Tests

Performance tests use Terraform to test large scale deployments of K3s clusters.

See [perf/README.md](./perf/README.md) for more info.
___

## End-to-End (E2E) Tests

E2E tests cover multi-node K3s configuration and administration: bringup, update, teardown etc. across a wide range of operating systems. E2E tests are run nightly as part of K3s quality assurance (QA).

See [e2e/README.md](./e2e/README.md) for more info.

___

## Distros Test Framework

The acceptance tests from distros test framework are a customizable way to create clusters and perform validations on them such that the requirements of specific features and functions can be validated.

See [distros-test-framework/README](https://github.com/rancher/distros-test-framework#readme) for more info.
___

## Contributing New Or Updated Tests

We gladly accept new and updated tests of all types. If you wish to create
a new test or update an existing test, please submit a PR with a title that includes the words `<NAME_OF_TEST> (Created/Updated)`.
