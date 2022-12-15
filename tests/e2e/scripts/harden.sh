#!/bin/bash

echo "vm.panic_on_oom=0
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
kernel.keys.root_maxbytes=25000000
" >> /etc/sysctl.d/90-kubelet.conf
sysctl -p /etc/sysctl.d/90-kubelet.conf

mkdir -p /var/lib/rancher/k3s/server
mkdir -m 700 /var/lib/rancher/k3s/server/logs
echo "apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata" >> /var/lib/rancher/k3s/server/audit.yaml

if [ "$1" = "psa" ]; then
    echo "apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: PodSecurity
  configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1beta1
    kind: PodSecurityConfiguration
    defaults:
      enforce: \"restricted\"
      enforce-version: \"latest\"
      audit: \"restricted\"
      audit-version: \"latest\"
      warn: \"restricted\"
      warn-version: \"latest\"
    exemptions:
      usernames: []
      runtimeClasses: []
      namespaces: [kube-system, cis-operator-system]" >> /var/lib/rancher/k3s/server/psa.yaml
fi