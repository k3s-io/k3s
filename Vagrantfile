BOX = "generic/alpine39"
HOME = File.dirname(__FILE__)
PROJECT = File.basename(HOME)
MOUNT_TYPE = ENV['MOUNT_TYPE'] || "nfs"
NUM_NODES = (ENV['NUM_NODES'] || 0).to_i
NODE_CPUS = (ENV['NODE_CPUS'] || 4).to_i
NODE_MEMORY = (ENV['NODE_MEMORY'] || 8192).to_i
NETWORK_PREFIX = ENV['NETWORK_PREFIX'] || "10.135.135"
VAGRANT_PROVISION = ENV['VAGRANT_PROVISION'] || "./scripts/vagrant-provision"

# --- Rules for /etc/sudoers to avoid password entry configuring NFS:
# %admin	ALL = (root) NOPASSWD: /usr/bin/sed -E -e * -ibak /etc/exports
# %admin	ALL = (root) NOPASSWD: /usr/bin/tee -a /etc/exports
# %admin	ALL = (root) NOPASSWD: /sbin/nfsd restart
# --- May need to add terminal to System Preferences -> Security & Privacy -> Privacy -> Full Disk Access

# --- Check for missing plugins
required_plugins = %w( vagrant-alpine vagrant-timezone )
plugin_installed = false
required_plugins.each do |plugin|
  unless Vagrant.has_plugin?(plugin)
    system "vagrant plugin install #{plugin}"
    plugin_installed = true
  end
end
# --- If new plugins installed, restart Vagrant process
if plugin_installed === true
  exec "vagrant #{ARGV.join' '}"
end

provision = <<SCRIPT
# --- Use system gopath if available
export GOPATH=#{ENV['GOPATH']}
# --- Default to root user for vagrant ssh
cat <<\\EOF >/etc/profile.d/root.sh
[ $EUID -ne 0 ] && exec sudo -i
EOF
# --- Set home to current directory
cat <<\\EOF >/etc/profile.d/home.sh
export HOME="#{HOME}" && cd
EOF
. /etc/profile.d/home.sh
# --- Run vagrant provision script if available
if [ ! -x #{VAGRANT_PROVISION} ]; then
  echo 'WARNING: Unable to execute provision script "#{VAGRANT_PROVISION}"'
  exit
fi
echo "running '#{VAGRANT_PROVISION}'..." && \
  #{VAGRANT_PROVISION} && \
  echo "finished '#{VAGRANT_PROVISION}'!"
SCRIPT

Vagrant.configure("2") do |config|
  config.vm.provider "virtualbox" do |v|
    v.cpus = NODE_CPUS
    v.memory = NODE_MEMORY
    v.customize ["modifyvm", :id, "--audio", "none"]
  end

  config.vm.box = BOX
  config.vm.hostname = PROJECT
  config.vm.synced_folder ".", HOME, type: MOUNT_TYPE
  config.vm.provision "shell", inline: provision
  config.timezone.value = :host

  config.vm.network "private_network", ip: "#{NETWORK_PREFIX}.100" if NUM_NODES==0

  (1..NUM_NODES).each do |i|
    config.vm.define ".#{i}" do |node|
      node.vm.network "private_network", ip: "#{NETWORK_PREFIX}.#{100+i}"
      node.vm.hostname = "#{PROJECT}-#{i}"
    end
  end
end
