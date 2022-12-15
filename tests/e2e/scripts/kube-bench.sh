#!/bin/bash
LATEST_VERSION=$(curl --silent "https://api.github.com/repos/aquasecurity/kube-bench/releases/latest" |  jq -r .tag_name)
curl -LO "https://github.com/aquasecurity/kube-bench/releases/download/${LATEST_VERSION}/kube-bench_${LATEST_VERSION/v/}_linux_amd64.tar.gz"
mkdir -p /etc/kube-bench
tar -xvf kube-bench_"${LATEST_VERSION/v/}"_linux_amd64.tar.gz -C /etc/kube-bench
rm kube-bench_"${LATEST_VERSION/v/}"_linux_amd64.tar.gz
mv /etc/kube-bench/kube-bench /usr/local/bin

# Add our k3s 1.23 benchmark
echo "  \"cis-1.23-k3s\":
    - \"master\"
    - \"node\"
    - \"controlplane\"
    - \"etcd\"
    - \"policies\"" >> /etc/kube-bench/cfg/config.yaml

mkdir -p /etc/kube-bench/cfg/cis-1.23-k3s
SECURITY_REPO="https://raw.githubusercontent.com/rancher/security-scan/master/package"
curl -L "$SECURITY_REPO"/cfg/k3s-cis-1.23-permissive/config.yaml -o /etc/kube-bench/cfg/cis-1.23-k3s/config.yaml
curl -L "$SECURITY_REPO"/cfg/k3s-cis-1.23-permissive/master.yaml -o /etc/kube-bench/cfg/cis-1.23-k3s/master.yaml
curl -L "$SECURITY_REPO"/cfg/k3s-cis-1.23-permissive/node.yaml -o /etc/kube-bench/cfg/cis-1.23-k3s/node.yaml
curl -L "$SECURITY_REPO"/cfg/k3s-cis-1.23-permissive/controlplane.yaml -o /etc/kube-bench/cfg/cis-1.23-k3s/controlplane.yaml
curl -L "$SECURITY_REPO"/cfg/k3s-cis-1.23-permissive/etcd.yaml -o /etc/kube-bench/cfg/cis-1.23-k3s/etcd.yaml
curl -L "$SECURITY_REPO"/cfg/k3s-cis-1.23-permissive/policies.yaml -o /etc/kube-bench/cfg/cis-1.23-k3s/policies.yaml
curl -L "$SECURITY_REPO"/helper_scripts/check_for_k3s_etcd.sh -o /usr/bin/check_for_k3s_etcd.sh
chmod +x /usr/bin/check_for_k3s_etcd.sh
curl -L "$SECURITY_REPO"/helper_scripts/check_for_k3s_network_policies.sh -o /usr/bin/check_for_k3s_network_policies.sh
chmod +x /usr/bin/check_for_k3s_network_policies.sh

