#!/bin/bash
node_ip=$1

curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
helm repo add jetstack https://charts.jetstack.io
helm repo update
kubectl create namespace cattle-system
kubectl create namespace cert-manager
kubectl apply --validate=false -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.crds.yaml

helm install cert-manager jetstack/cert-manager --namespace cert-manager
kubectl get pods --namespace cert-manager
helm install rancher rancher-latest/rancher --namespace cattle-system --set hostname="$node_ip".nip.io
