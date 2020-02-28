OS = (ENV['OS'] || "alpine310")
BOX_REPO = (ENV['BOX_REPO'] || "generic")
BOX = (ENV['BOX'] || "#{BOX_REPO}/#{OS}")
HOME = File.dirname(__FILE__)
PROJECT = File.basename(HOME)
NUM_NODES = (ENV['NUM_NODES'] || 0).to_i
NODE_CPUS = (ENV['NODE_CPUS'] || 4).to_i
NODE_MEMORY = (ENV['NODE_MEMORY'] || 8192).to_i
NETWORK_PREFIX = ENV['NETWORK_PREFIX'] || "10.135.135"
VAGRANT_PROVISION = ENV['VAGRANT_PROVISION'] || "./scripts/provision/vagrant"
MOUNT_TYPE = ENV['MOUNT_TYPE'] || "nfs"

# --- Rules for /etc/sudoers to avoid password entry configuring NFS:
# %admin	ALL = (root) NOPASSWD: /usr/bin/sed -E -e * -ibak /etc/exports
# %admin	ALL = (root) NOPASSWD: /usr/bin/tee -a /etc/exports
# %admin	ALL = (root) NOPASSWD: /sbin/nfsd restart
# --- May need to add terminal to System Preferences -> Security & Privacy -> Privacy -> Full Disk Access

def provision(vm)
  vm.provision "shell",
      path: VAGRANT_PROVISION,
      env: { 'HOME' => HOME, 'GOPATH' => ENV['GOPATH'], 'BOX' => vm.box }
end

Vagrant.configure("2") do |config|

  config.vm.provider "virtualbox" do |v|
    v.cpus = NODE_CPUS
    v.memory = NODE_MEMORY
    v.customize ["modifyvm", :id, "--audio", "none"]
  end

  config.vm.box = BOX
  config.vm.hostname = PROJECT
  config.vm.synced_folder ".", HOME, type: MOUNT_TYPE

  if Vagrant.has_plugin?("vagrant-timezone")
    config.timezone.value = :host
  end

  if NUM_NODES==0
    config.vm.network "private_network", ip: "#{NETWORK_PREFIX}.100"
    provision(config.vm)
  else
    (1..NUM_NODES).each do |i|
      config.vm.define ".#{i}" do |node|
        node_os = (ENV["OS_#{i}"] || OS)
        node.vm.box = (ENV["BOX_#{i}"] || "#{BOX_REPO}/#{node_os}")
        node.vm.network "private_network", ip: "#{NETWORK_PREFIX}.#{100+i}"
        node.vm.hostname = "#{PROJECT}-#{i}"
        provision(node.vm)
      end
    end
  end

end
