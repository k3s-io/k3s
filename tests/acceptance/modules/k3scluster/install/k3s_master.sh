#!/bin/bash

mkdir -p /etc/rancher/k3s
sudo mkdir -p -m 700 /var/lib/rancher/k3s/server/logs
mkdir -p /var/lib/rancher/k3s/server/manifests

cat <<EOF >>/etc/rancher/k3s/config.yaml
write-kubeconfig-mode: "0644"
tls-san:
  - ${2}
EOF

if [[ -n "$8" ]] && [[ "$8" == *":"* ]]
then
   echo "$"
   echo -e "$8" >> /etc/rancher/k3s/config.yaml
   cat /etc/rancher/k3s/config.yaml
fi

if [[ -n "${8}" ]] && [[ "${8}" == *"protect-kernel-defaults"* ]]
then
   cat /tmp/cis_master_config.yaml >> /etc/rancher/k3s/config.yaml
   printf "%s\n" "vm.panic_on_oom=0" "vm.overcommit_memory=1" "kernel.panic=10" "kernel.panic_on_oops=1" "kernel.keys.root_maxbytes=25000000" >> /etc/sysctl.d/90-kubelet.conf
   sysctl -p /etc/sysctl.d/90-kubelet.conf
   systemctl restart systemd-sysctl
   cat /tmp/policy.yaml > /var/lib/rancher/k3s/server/manifests/policy.yaml
   cat /tmp/audit.yaml > /var/lib/rancher/k3s/server/audit.yaml
   cat /tmp/cluster-level-pss.yaml > /var/lib/rancher/k3s/server/cluster-level-pss.yaml
  if [[ "${4}" == *"v1.18"* ]] || [[ "${4}" == *"v1.19"* ]] || [[ "${4}" == *"v1.20"* ]]
  then
    cat /tmp/v120ingresspolicy.yaml > /var/lib/rancher/k3s/server/manifests/v120ingresspolicy.yaml
  else
    cat /tmp/v121ingresspolicy.yaml > /var/lib/rancher/k3s/server/manifests/v121ingresspolicy.yaml
  fi
fi

if [[ "${8}" == *"traefik"* ]]
then
   mkdir -p /var/lib/rancher/k3s/server/manifests
   cat /tmp/nginx-ingress.yaml> /var/lib/rancher/k3s/server/manifests/nginx-ingress.yaml
fi

if [ "${1}" = "rhel" ]
then
   subscription-manager register --auto-attach --username="${9}" --password="${10}"
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi


export "${3}"="${4}"
if [ "${5}" = "etcd" ]
then
   echo "CLUSTER TYPE is ETCD and channel is ${11}"
   if [[ "${4}" == *"v1.18"* ]] || [[ "${4}" == *"v1.17"* ]] && [[ -n "${8}" ]]
   then
       curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - server --cluster-init --node-external-ip="${6}" ${8} --tls-san "${2}" --write-kubeconfig-mode "0644"
   else
       if [[ -n "${11}" ]]
       then
           curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=${11} INSTALL_K3S_TYPE='server' sh -s - server --cluster-init --node-external-ip="${6}"
       else
           curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - server --cluster-init --node-external-ip="${6}"
       fi
   fi
else
  echo "CLUSTER TYPE is external db and channel is ${11}"
  if [[ "${4}" == *"v1.18"* ]] || [[ "${4}" == *"v1.17"* ]] && [[ -n "${8}" ]]
  then
      curl -sfL https://get.k3s.io | sh -s - server --node-external-ip="${6}" --datastore-endpoint="${7}" ${8} --tls-san "${2}" --write-kubeconfig-mode "0644"
  else
      if [[ -n "${11}" ]]
      then
          curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=${11} sh -s - server --node-external-ip="${6}" --datastore-endpoint="${7}"
      else
          curl -sfL https://get.k3s.io | sh -s - server --node-external-ip="${6}" --datastore-endpoint="${7}"
      fi
  fi
fi

export PATH=$PATH:/usr/local/bin
timeElapsed=0
while ! $(kubectl get nodes >/dev/null 2>&1) && [[ $timeElapsed -lt 300 ]]
do
   sleep 5
   timeElapsed=$(expr $timeElapsed + 5)
done

IFS=$'\n'
timeElapsed=0
sleep 10
while [[ $timeElapsed -lt 420 ]]
do
   notready=false
   for rec in $(kubectl get nodes)
   do
      if [[ "$rec" == *"NotReady"* ]]
      then
         notready=true
      fi
  done
  if [[ $notready == false ]]
  then
     break
  fi
  sleep 20
  timeElapsed=$(expr $timeElapsed + 20)
done

IFS=$'\n'
timeElapsed=0
while [[ $timeElapsed -lt 420 ]]
do
   helmPodsNR=false
   systemPodsNR=false
   for rec in $(kubectl get pods -A --no-headers)
   do
      if [[ "$rec" == *"helm-install"* ]] && [[ "$rec" != *"Completed"* ]]
      then
         helmPodsNR=true
      elif [[ "$rec" != *"helm-install"* ]] && [[ "$rec" != *"Running"* ]]
      then
         systemPodsNR=true
      else
         echo ""
      fi
   done

   if [[ $systemPodsNR == false ]] && [[ $helmPodsNR == false ]]
   then
      break
   fi
   sleep 20
   timeElapsed=$(expr $timeElapsed + 20)
done

cat /etc/rancher/k3s/config.yaml> /tmp/joinflags
cat /var/lib/rancher/k3s/server/node-token >/tmp/nodetoken
cat /etc/rancher/k3s/k3s.yaml >/tmp/config
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
sudo chmod 644 /etc/rancher/k3s/k3s.yaml
