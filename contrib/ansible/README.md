# Build a Kubernetes cluster using k3s via Ansible.

## Kubeadm Ansible Playbook

Build a Kubernetes cluster using Ansible with kubeadm. The goal is easily install a Kubernetes cluster on machines running:

- [X] Debian 9
- [ ] Ubuntu 16.04
- [ ] CentOS 7

on processor architecture:

- [X] x64
- [X] arm64
- [X] armhf

## System requirements:

Deployment environment must have Ansible 2.4.0+
Master and nodes must have passwordless SSH access

## Usage

Add the system information gathered above into a file called hosts.ini. For example:

```
[master]
192.16.35.12

[node]
192.16.35.[10:11]

[kube-cluster:children]
master
node
```

Start provisionning of the cluster using the following command:

```
ansible-playbook site.yaml
```

