DISTRO = (ENV['DISTRO'] || "alpine312")
BOX_REPO = (ENV['BOX_REPO'] || "generic")
HOME = ENV['HOME']
PROJ_HOME = File.dirname(__FILE__)
PROJECT = File.basename(PROJ_HOME)
NUM_NODES = (ENV['NUM_NODES'] || 0).to_i
NODE_CPUS = (ENV['NODE_CPUS'] || 4).to_i
NODE_MEMORY = (ENV['NODE_MEMORY'] || 8192).to_i
NETWORK_PREFIX = ENV['NETWORK_PREFIX'] || "10.135.135"
VAGRANT_PROVISION = ENV['VAGRANT_PROVISION'] || "./scripts/provision/vagrant"
MOUNT_TYPE = ENV['MOUNT_TYPE'] || "virtualbox"

# --- Rules for /etc/sudoers to avoid password entry configuring NFS:
# %admin	ALL = (root) NOPASSWD: /usr/bin/sed -E -e * -ibak /etc/exports
# %admin	ALL = (root) NOPASSWD: /usr/bin/tee -a /etc/exports
# %admin	ALL = (root) NOPASSWD: /sbin/nfsd restart
# --- May need to add terminal to System Preferences -> Security & Privacy -> Privacy -> Full Disk Access

def provision(vm, node_num)
  node_os = (ENV["DISTRO_#{node_num}"] || DISTRO)
  vm.box = (ENV["BOX_#{node_num}"] || ENV["BOX"] || "#{BOX_REPO}/#{node_os}")
  vm.hostname = "#{PROJECT}-#{node_num}-#{vm.box.gsub(/^.*\//,"")}"
  vm.network "private_network", ip: "#{NETWORK_PREFIX}.#{100+node_num}"
  vm.provision "shell",
      path: VAGRANT_PROVISION,
      env: { 'HOME' => PROJ_HOME, 'GOPATH' => ENV['GOPATH'], 'BOX' => vm.box }
end

Vagrant.configure("2") do |config|

  config.vm.provider "virtualbox" do |v|
    v.cpus = NODE_CPUS
    v.memory = NODE_MEMORY
    v.customize ["modifyvm", :id, "--audio", "none"]
  end
  config.vm.provider "libvirt" do |v|
    v.cpus = NODE_CPUS
    v.memory = NODE_MEMORY
  end
  if Vagrant.has_plugin?("vagrant-timezone")
    config.timezone.value = :host
  end
  if "#{MOUNT_TYPE}" == "nfs"
    config.vm.synced_folder HOME, HOME, type: "nfs", mount_options: ["vers=3,tcp"]
  else
    config.vm.synced_folder HOME, HOME, type: MOUNT_TYPE
  end

  if NUM_NODES==0
    provision(config.vm, 0)
  else
    (1..NUM_NODES).each do |i|
      config.vm.define ".#{i}" do |node|
        provision(node.vm, i)
      end
    end
  end

end
