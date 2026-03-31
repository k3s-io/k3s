#!/bin/sh
# Script to install pre-requisites on vsphere node to run e2e test

export PATH=$PATH:/usr/local/go/bin/:/usr/local/bin
echo 'Installing Kubectl'
KUBECTL_VERSION=v1.34.6
KUBECTL_SHA256=3166155b17198c0af34ff5a360bd4d9d58db98bafadc6f3c2a57ae560563cd6
if ! curl -fsSLo kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" || \
   ! echo "${KUBECTL_SHA256}  kubectl" | sha256sum --check --status; then
    echo "[ERROR] kubectl download or SHA256 verification failed"
    exit 1
fi
rm -f kubectl.sha256
sudo mv kubectl /usr/local/bin/
chmod a+x /usr/local/bin/kubectl

echo 'Installing jq and docker'
sudo apt-get -y install jq docker.io

echo 'Installing Go'
GO_VERSION=1.25.8
GO_SHA256=ceb5e041bbc3893846bd1614d76cb4681c91dadee579426cf21a63f2d7e03be6
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
if ! curl -fsSLo "${GO_TARBALL}" "https://dl.google.com/go/${GO_TARBALL}" || \
   ! echo "${GO_SHA256}  ${GO_TARBALL}" | sha256sum --check --status; then
    echo "[ERROR] go download or SHA256 verification failed"
    exit 1
fi
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf "${GO_TARBALL}"
rm -f "${GO_TARBALL}"
echo 
go version

echo 'Installing Virtualbox'
sudo apt-get -y update && sudo apt-get -y upgrade
sudo apt-get -y install virtualbox
vboxmanage --version

# Virtualbox >= 6.1.28 require `/etc/vbox/network.conf
ls /etc/vbox 2>/dev/null || sudo mkdir -p /etc/vbox
echo "* 10.0.0.0/8 192.168.0.0/16">>/etc/vbox/networks.conf

echo 'Installing vagrant'
sudo apt-get -y install -f unzip
VAGRANT_VERSION=2.2.19
VAGRANT_ZIP="vagrant_${VAGRANT_VERSION}_linux_amd64.zip"
VAGRANT_SHA256=a1df4c793902e2b9647a0fd42a23d2363c6900f54b70674b736898f9e48c1200
curl -fsSLO "https://releases.hashicorp.com/vagrant/${VAGRANT_VERSION}/${VAGRANT_ZIP}"
if ! echo "${VAGRANT_SHA256}  ${VAGRANT_ZIP}" | sha256sum --check --status; then
    echo "[ERROR] vagrant download or SHA256 verification failed"
    exit 1
fi
unzip "${VAGRANT_ZIP}"
sudo mv vagrant /usr/local/bin/
rm -f "${VAGRANT_ZIP}"
vagrant --version
sudo apt-get -y install libarchive-tools
vagrant plugin install vagrant-k3s vagrant-reload vagrant-scp

echo 'Cloning repo'
ls k3s 2>/dev/null || git clone https://github.com/k3s-io/k3s.git

# Use curl -X GET <IP_ADDR>:5000/v2/_catalog to see cached images
echo 'Setting up docker registry as a cache'
docker run -d -p 5000:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
    --restart always \
    --name registry registry:2