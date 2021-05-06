#!/bin/bash
# Wait for CoreDNS pods to be ready.

set -x
echo "Waiting for CoreDNS pods to be ready..."
counter=0
# `kubectl wait` fails when the pods with the specified label are not created yet
until kubectl wait --for=condition=ready pods --namespace=kube-system -l k8s-app=kube-dns; do
  ((counter++))
  if [[ $counter -eq 20 ]]; then
    echo "CoreDNS not running?"
    kubectl get pods -A
    kubectl get nodes -o wide
    exit 1
  fi
  sleep 10
done
