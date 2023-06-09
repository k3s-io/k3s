---
name: Validate Operating System
about: Request validation of an operating system
title: 'Validate OS VERSION'
labels: ["kind/os-validation"]
assignees: ''

---

<!-- Thanks for helping us to improve K3s! We welcome all OS validation requests. Please fill out each area of the template so we can better help you. Comments like this will be hidden when you post but you can delete them if you wish. -->

**K3s Versions to be Validated**
<!-- A list of released k3s versions to validate the OS for. -->


**Testing Considerations**
<!-- Add/remove test cases that should be considered in addition/as opposed to to the below standard ones. -->
1. Install and run sonobuoy conformance tests on a hardened cluster
2. Validate SUC upgrade
3. Install Rancher Manager
4. Validate snapshot restore via `cluster-reset-restore-path`


**Additional Information**
<!-- Add any other information or context about the OS here. Ex. "Please validate with Selinux [...]" -->

