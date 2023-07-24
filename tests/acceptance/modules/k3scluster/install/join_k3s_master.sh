#!/bin/bash
# This script is used to join one or more nodes as masters

mkdir -p /etc/rancher/k3s
sudo mkdir -p -m 700 /var/lib/rancher/k3s/server/logs
mkdir -p /var/lib/rancher/k3s/server/manifests

cat <<EOF >>/etc/rancher/k3s/config.yaml
write-kubeconfig-mode: "0644"
tls-san:
  - ${2}
token: "${8}"
EOF

if [[ -n "${10}" ]] && [[ "${10}" == *":"* ]]
then
   echo -e "${10}" >> /etc/rancher/k3s/config.yaml
   cat /etc/rancher/k3s/config.yaml
fi
if [ "${1}" = "rhel" ]
then
   subscription-manager register --auto-attach --username="${11}" --password="${12}"
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

if [[ -n "${10}" ]] && [[ "${10}" == *"protect-kernel-defaults"* ]]
then
  cat /tmp/cis_master_config.yaml >> /etc/rancher/k3s/config.yaml
  printf "%s\n" "vm.panic_on_oom=0" "vm.overcommit_memory=1" "kernel.panic=10" "kernel.panic_on_oops=1" "kernel.keys.root_maxbytes=25000000" >> /etc/sysctl.d/90-kubelet.conf
  sysctl -p /etc/sysctl.d/90-kubelet.conf
  systemctl restart systemd-sysctl
  cat /tmp/audit.yaml > /var/lib/rancher/k3s/server/audit.yaml
  cat /tmp/policy.yaml > /var/lib/rancher/k3s/server/manifests/policy.yaml
  cat /tmp/cluster-level-pss.yaml > /var/lib/rancher/k3s/server/cluster-level-pss.yaml
  if [[ "${4}" == *"v1.18"* ]] || [[ "${4}" == *"v1.19"* ]] || [[ "${4}" == *"v1.20"* ]]
  then
    cat /tmp/v120ingresspolicy.yaml > /var/lib/rancher/k3s/server/manifests/v120ingresspolicy.yaml
  else
    cat /tmp/v121ingresspolicy.yaml > /var/lib/rancher/k3s/server/manifests/v121ingresspolicy.yaml
  fi
fi


export "${3}"="${4}"
if [ "${5}" = "etcd" ]
then
    if [[ "${4}" == *"v1.18"* ]] || [[ "${4}" == *"v1.17"* ]] && [[ -n "${10}" ]]
    then
        curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - server --server https://"${7}":6443 --token "${8}" --node-external-ip="${6}" --tls-san "${2}" --write-kubeconfig-mode "0644"
    else
        if [[ -n  "${13}" ]]
        then
          curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=${13} INSTALL_K3S_TYPE='server' sh -s - server --server https://"${7}":6443 --node-external-ip="${6}"
        else
          curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - server --server https://"${7}":6443 --node-external-ip="${6}"
        fi
    fi
else
   if [[ "${4}" == *"v1.18"* ]] || [[ "${4}" == *"v1.17"* ]] && [[ -n "${10}" ]]
    then
        curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - server --node-external-ip="${6}" --datastore-endpoint="${9}" --tls-san "${2}" --write-kubeconfig-mode "0644"
    else
        if [[ -n "${13}" ]]
        then
          curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=${13} INSTALL_K3S_TYPE='server' sh -s - server --node-external-ip="${6}" --datastore-endpoint="${9}"
        else
          curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - server --node-external-ip="${6}" --datastore-endpoint="${9}"
        fi
    fi
fi