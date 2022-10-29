#!/bin/bash
node_ip=$1

echo  "Give K3s time to startup"
sleep 10
kubectl -n kube-system rollout status deploy/coredns
kubectl -n kube-system rollout status deploy/local-path-provisioner

cat << EOF > /var/lib/rancher/k3s/server/manifests/rancher.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager
---
apiVersion: v1
kind: Namespace
metadata:
  name: cattle-system
---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  namespace: kube-system
  name: cert-manager
spec:
  targetNamespace: cert-manager
  version: v1.6.1
  chart: cert-manager
  repo: https://charts.jetstack.io
  set:
    installCRDs: "true"
---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  namespace: kube-system
  name: rancher
spec:
  targetNamespace: cattle-system
  version: 2.6.5
  chart: rancher
  repo: https://releases.rancher.com/server-charts/latest
  set:
    ingress.tls.source: "rancher"
    hostname: "$node_ip.nip.io"
    replicas: 1
EOF


echo "Give Rancher time to startup"
sleep 20
kubectl -n cert-manager rollout status deploy/cert-manager
while ! kubectl get secret --namespace cattle-system bootstrap-secret -o go-template='{{.data.bootstrapPassword|base64decode}}' &> /dev/null; do
    ((iterations++))
    if [ "$iterations" -ge 8 ]; then
        echo "Unable to find bootstrap-secret"
        exit 1
    fi
    echo "waiting for bootstrap-secret..."
    sleep 20
done
echo https://"$node_ip".nip.io/dashboard/?setup=$(kubectl get secret --namespace cattle-system bootstrap-secret -o go-template='{{.data.bootstrapPassword|base64decode}}')