# End-to-End (E2E) Tests

E2E tests cover multi-node K3s configuration and administration: bringup, update, teardown etc. across a wide range of operating systems. E2E tests are run nightly as part of K3s quality assurance (QA).

## Framework 
End-to-end tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the integration tests, but rely on [Vagrant](https://www.vagrantup.com/) to provide the underlying cluster configuration. 

Currently tested operating systems are:
- [Ubuntu 20.04](https://app.vagrantup.com/generic/boxes/ubuntu2004)
- [Leap 15.3](https://app.vagrantup.com/opensuse/boxes/Leap-15.3.x86_64) (stand-in for SLE-Server)
- [MicroOS](https://app.vagrantup.com/dweomer/boxes/microos.amd64) (stand-in for SLE-Micro)

## Format

All E2E tests should be placed under `tests/e2e/<TEST_NAME>`.  
All E2E test functions should be named: `Test_E2E<TEST_NAME>`.  
A E2E test consists of two parts:
1. `Vagrantfile`: a vagrant file which describes and configures the VMs upon which the cluster and test will run
2. `<TEST_NAME>.go`: A go test file which calls `vagrant up` and controls the actual testing of the cluster

See the [validate cluster test](../tests/e2e/validatecluster/validatecluster_test.go) as an example.

## Running

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

# Debugging
In the event of a test failure, the cluster and VMs are retained in their broken state. Startup logs are retained in `vagrant.log`.  
To see a list of nodes: `vagrant status`    
To ssh into a node: `vagrant ssh <NODE>`  
Once you are done/ready to restart the test, use `vagrant destroy -f` to remove the broken cluster.  