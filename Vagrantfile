# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
  config.vm.box = "bento/ubuntu-19.04"
  config.vm.provider "virtualbox" do |v|
    v.cpus = 4
    v.memory = 8192
  end

  config.vm.synced_folder ".", "/go/src/k8s.io/kubernetes/"

  config.vm.provision "shell", inline: <<-SHELL
    set -e -x -u
    apt-get update -y || (sleep 40 && apt-get update -y)
    apt-get install -y git gcc-multilib gcc-mingw-w64 jq git-secrets
    wget -qO- https://storage.googleapis.com/golang/go1.12.7.linux-amd64.tar.gz | tar -C /usr/local -xz
    echo 'export GOPATH=/go' >> /root/.bashrc
    echo 'ulimit -n 65535' >> /root/.bashrc
    GOPATH=/go /usr/local/go/bin/go get github.com/rancher/trash && rm -rf /go/src/github.com/rancher
    echo 'export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin' >> /root/.bashrc
  SHELL
end
