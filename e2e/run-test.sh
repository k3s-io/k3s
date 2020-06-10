#!/bin/bash
set -e -x

while [ ! -e /etc/rancher/k3s/k3s.yaml ]; do
    echo waiting for config
    sleep 1
done

mkdir -p /root/.kube
sed 's/localhost/server/g' /etc/rancher/k3s/k3s.yaml > /root/.kube/config
export KUBECONFIG=/root/.kube/config
cat /etc/rancher/k3s/k3s.yaml
cat $KUBECONFIG
sonobuoy run
sleep 15
sonobuoy logs -f
