# Development Guide

Since K3s is written in Go, it is fair to assume that the Go tools are all one needs to contribute to this project. Unfortunately, there is a point where this no longer holds true when required to test or build local changes. This document elaborates on the required tooling for K3s development.

- [Non-Linux environment prerequisites](#non-linux-environment-prerequisites)
  - [Windows Setup](#windows-setup)
  - [macOS Setup](#macos-setup)
- [Installing Required Software](#installing-required-software)
  - [Go](#go)
  - [Docker](#docker)
  - [Vagrant](#vagrant)
- [Cloning, Building and Testing K3s](#cloning-building-and-testing-k3s)
- [Dependency management](#dependency-management)

## Non-Linux environment prerequisites

All the test and build scripts within this repository were created to be run on GNU Linux development environments. Due to this, it is suggested to use the virtual machine defined on this repository's [Vagrantfile](../../Vagrantfile) to use them.

Either way, if one still wants to build and test K3s on non-Linux environments, specific setups are to be followed.

### Windows Setup

To build K3s on Windows is only possible for versions that support Windows Subsystem for Linux (WSL). If the development environment in question has Windows 10, Version 2004, Build 19041 or higher, [follow these instructions to install WSL2](https://docs.microsoft.com/en-us/windows/wsl/install-win10); otherwise, use a Linux Virtual machine instead.

### macOS Setup

The shell scripts in charge of the build and test processes rely on GNU utils (i.e. `sed`), [which slightly differ on macOS](https://unix.stackexchange.com/a/79357), meaning that one must make some adjustments before using them.

First, install the GNU utils:

```sh
brew install coreutils findutils gawk gnu-sed gnu-tar grep make
```

Then update the shell init script (i.e. `.bashrc`) to prepend the GNU Utils to the `$PATH` variable

```sh
GNUBINS="$(find /usr/local/opt -type d -follow -name gnubin -print)"

for bindir in ${GNUBINS[@]}; do
  PATH=$bindir:$PATH
done

export PATH
```

## Installing Required Software

### Go

It is well known that K3s is written in [Go](http://golang.org). Please follow the [Go Getting Started guide](https://golang.org/doc/install) to install and set up the Go tools used to compile and run the test batteries.

**Note:** K3s uses the same Go version as the Kubernetes components underneath. The table below lists the required Go versions for supported the Kubernetes releases.

| Kubernetes     | requires Go |
|----------------|-------------|
| 1.19 - 1.20    | 1.15.5      |
| 1.21 - 1.22    | 1.16.7      |
| 1.23+          | 1.17        |

### Docker

K3s build and test processes development require Docker to run certain steps. [Follow the Docker website instructions to install Docker](https://docs.docker.com/get-docker/) in the development environment.

### Vagrant

As described in the [Testing documentation](../../tests/TESTING.md), all the smoke tests are run in virtual machines managed by Vagrant.  To install Vagrant in the development environment, [follow the instructions from the Hashicorp website](https://www.vagrantup.com/downloads), alongside any of the following hypervisors:

- [VirtualBox](https://www.virtualbox.org/)
- [libvirt](https://libvirt.org/) and the [vagrant-libvirt plugin](https://github.com/vagrant-libvirt/vagrant-libvirt#installation)

## Cloning, Building and Testing K3s

These topics already have been addressed on their respective documents:

- [Git Workflow](./git-workflow.md)
- [Building](../../BUILDING.md)
- [Testing](../../tests/TESTING.md)

## Dependency management

K3s uses [go modules](https://github.com/golang/go/wiki/Modules) to manage dependencies.
