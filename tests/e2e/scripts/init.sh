#!/bin/sh
# Script to install pre-requisites on vsphere node to run e2e test

export PATH=$PATH:/usr/local/go/bin/:/usr/local/bin
echo 'Installing Kubectl'
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
sudo mv kubectl /usr/local/bin/ && \
chmod a+x /usr/local/bin/kubectl

echo 'Installing jq'
sudo apt-get -y install jq

echo 'Installing Go'
curl -L https://dl.google.com/go/go1.16.10.linux-amd64.tar.gz | tar xz
sudo mv go /usr/local
/usr/local/go/bin/go version
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
sudo cp vagrant /usr/local/bin/
vagrant --version
sudo apt-get -y install libarchive-tools
vagrant plugin install vagrant-k3s
vagrant plugin install vagrant-reload

echo 'Cloning repo'
ls k3s 2>/dev/null || git clone https://github.com/k3s-io/k3s.git
