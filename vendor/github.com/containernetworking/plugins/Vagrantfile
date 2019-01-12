# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
  config.vm.box = "bento/ubuntu-16.04"

  config.vm.synced_folder "..", "/go/src/github.com/containernetworking"

  config.vm.provision "shell", inline: <<-SHELL
    set -e -x -u

    apt-get update -y || (sleep 40 && apt-get update -y)
    apt-get install -y git

    wget -qO- https://storage.googleapis.com/golang/go1.9.1.linux-amd64.tar.gz | tar -C /usr/local -xz

    echo 'export GOPATH=/go' >> /root/.bashrc
    echo 'export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin' >> /root/.bashrc
    cd /go/src/github.com/containernetworking/plugins
  SHELL
end
