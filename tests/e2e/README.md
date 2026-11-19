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

See the [validate cluster test](./validatecluster/validatecluster_test.go) as an example.


## Setup

To run the E2E tests, you must first install the following:
- Vagrant
- Libvirt
- Vagrant plugins

### Vagrant 

Download the latest version (currently 2.2.19) of Vagrant [*from the website*](https://www.vagrantup.com/downloads). Do not use built-in packages, they often old or do not include the required ruby library extensions necessary to get certain plugins working.

### Libvirt
Follow the OS specific guides to install libvirt/qemu on your host:  
- [openSUSE](https://documentation.suse.com/sles/15-SP1/html/SLES-all/cha-vt-installation.html)  
- [ubuntu 20.04](https://ubuntu.com/server/docs/virtualization-libvirt)  
- ubuntu 22.04: 
  ```bash
  sudo apt install ruby-libvirt qemu libvirt-daemon-system libvirt-clients ebtables dnsmasq-base libxslt-dev libxml2-dev libvirt-dev zlib1g-dev ruby-dev libguestfs-tools
  ```
- ubuntu 24.04:
  ```bash
  sudo apt install ruby-libvirt qemu-kvm libvirt-daemon-system libvirt-clients ebtables dnsmasq-base libxslt-dev libxml2-dev libvirt-dev zlib1g-dev ruby-dev libguestfs-tools
  ```
- [debian](https://wiki.debian.org/KVM#Installation)  
- [fedora](https://developer.fedoraproject.org/tools/virtualization/installing-libvirt-and-virt-install-on-fedora-linux.html)

### Vagrant plugins
Install the necessary vagrant plugins with the following command:

```bash
vagrant plugin install vagrant-libvirt vagrant-scp vagrant-k3s vagrant-reload
```
### Kubectl

For linux
```bash
   curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
   sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
```
If it does not work, or you are on a different system, check the [official tutorial](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)

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

### Windows agent

This suite requires a Windows Server 2025 Vagrant box and checks HTTP connectivity in both directions between Linux and Windows pods.

Build the Windows binary from the same release tag:

```bash
export E2E_RELEASE_VERSION='v1.37.1+k3s1'
git clone --depth 1 --branch "$E2E_RELEASE_VERSION" https://github.com/k3s-io/k3s.git /tmp/k3s-release-source
cd /tmp/k3s-release-source
docker buildx build --platform=windows/amd64 --target=result \
  --build-arg GIT_TAG="$E2E_RELEASE_VERSION" \
  --output=type=local,dest=/tmp/k3s-release-artifacts -f Dockerfile .
export E2E_WINDOWS_K3S_BIN=/tmp/k3s-release-artifacts/dist/artifacts/k3s.exe
export E2E_WINDOWS_BOX=gusztavvargadr/windows-server-2025-standard

cd /path/to/k3s/tests/e2e/windows
go test -v -timeout=75m .
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