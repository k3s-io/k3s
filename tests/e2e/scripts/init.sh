#!/bin/sh
# Script to install pre-requisites on vsphere node to run e2e test

export PATH=$PATH:/usr/local/go/bin/:/usr/local/bin
echo 'Installing Kubectl'
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
sudo mv kubectl /usr/local/bin/ && \
chmod a+x /usr/local/bin/kubectl

echo 'Installing jq and docker'
sudo apt-get -y install jq docker.io

echo 'Installing Go'
GO_VERSION=1.19.1
wget  --quiet https://dl.google.com/go/go$GO_VERSION.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go$GO_VERSION.linux-amd64.tar.gz
rm go$GO_VERSION.linux-amd64.tar.gz
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
curl -O https://releases.hashicorp.com/vagrant/2.2.19/vagrant_2.2.19_linux_amd64.zip
unzip vagrant_2.2.19_linux_amd64.zip
sudo mv vagrant /usr/local/bin/
rm vagrant_2.2.19_linux_amd64.zip
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