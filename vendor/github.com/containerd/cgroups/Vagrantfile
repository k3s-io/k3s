# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
# Fedora box is used for testing cgroup v2 support
  config.vm.box = "fedora/32-cloud-base"
  config.vm.provider :virtualbox do |v|
    v.memory = 2048
    v.cpus = 2
  end
  config.vm.provider :libvirt do |v|
    v.memory = 2048
    v.cpus = 2
  end
  config.vm.provision "shell", inline: <<-SHELL
    cat << EOF | dnf -y shell
config exclude kernel,kernel-core
config install_weak_deps false
update
install git golang-go
ts run
EOF
    dnf clean all
  SHELL
end
