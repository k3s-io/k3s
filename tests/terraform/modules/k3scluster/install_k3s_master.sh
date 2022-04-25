#!/bin/bash

mkdir -p /etc/rancher/k3s
cat << EOF >/etc/rancher/k3s/config.yaml
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

if [ "${1}" = "rhel" ]
then
   subscription-manager register --auto-attach --username="${9}" --password="${10}"
   subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

export "${3}"="${4}"

if [ "${5}" = "etcd" ]
then
   echo "CLUSTER TYPE  is etcd"
   if [[ "$4" == *"v1.18"* ]] || [["$4" == *"v1.17"* ]] && [[ -n "$8" ]]
   then
       echo "curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - --cluster-init --node-external-ip=${6} $8" >/tmp/master_cmd
       curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - --cluster-init --node-external-ip="${6}" "$8"
   else
       echo "curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - --cluster-init --node-external-ip=${6}" >/tmp/master_cmd
       curl -sfL https://get.k3s.io | INSTALL_K3S_TYPE='server' sh -s - --cluster-init --node-external-ip="${6}"
   fi
else
   echo "CLUSTER TYPE is external db"
   echo "$8"
   if [[ "$4" == *"v1.18"* ]] || [[ "$4" == *"v1.17"* ]] && [[ -n "$8" ]]
   then
       echo "curl -sfL https://get.k3s.io | sh -s - server --node-external-ip=${6} --datastore-endpoint=\"${7}\" $8"  >/tmp/master_cmd
       curl -sfL https://get.k3s.io | sh -s - server --node-external-ip="${6}" --datastore-endpoint="${7}" "$8"
   else
       echo "curl -sfL https://get.k3s.io | sh -s - server --node-external-ip=${6}  --datastore-endpoint=\"${7}\" "  >/tmp/master_cmd
       curl -sfL https://get.k3s.io | sh -s - server --node-external-ip="${6}" --datastore-endpoint="${7}"
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
