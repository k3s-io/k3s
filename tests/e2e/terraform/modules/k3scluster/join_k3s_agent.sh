#!/bin/bash
# This script is used to join one or more nodes as agents

mkdir -p /etc/rancher/k3s
cat <<EOF >>/etc/rancher/k3s/config.yaml
server: https://${4}:6443
token:  "${5}"
EOF

if [[ ! -z "$7" ]] && [[ "$7" == *":"* ]]
then
   echo -e "$7" >> /etc/rancher/k3s/config.yaml
   cat /etc/rancher/k3s/config.yaml
fi

if [ ${1} = "rhel" ]
then
    subscription-manager register --auto-attach --username=${8} --password=${9}
    subscription-manager repos --enable=rhel-7-server-extras-rpms
fi

export "${2}"="${3}"
  if [[ "$3" == *"v1.18"* ]] || [["$3" == *"v1.17"* ]] && [[ -n "$7" ]]
then
  echo "curl -sfL https://get.k3s.io | sh -s - agent --node-external-ip=${6} $7" >/tmp/agent_cmd
curl -sfL https://get.k3s.io | sh -s - agent --node-external-ip=${6} ${7}
  else

echo "curl -sfL https://get.k3s.io | sh -s - agent --node-external-ip=${6}" >/tmp/agent_cmd
curl -sfL https://get.k3s.io | sh -s - agent --node-external-ip=${6}
fi
