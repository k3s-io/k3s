def defaultOSConfigure(vm)
  box = vm.box.to_s
  if box.include?("generic/ubuntu")
    vm.provision "Set DNS", type: "shell", inline: "netplan set ethernets.eth0.nameservers.addresses=[8.8.8.8,1.1.1.1]; netplan apply", run: 'once'
    vm.provision "Install jq", type: "shell", inline: "apt install -y jq"
  elsif box.include?("Leap") || box.include?("Tumbleweed")
    vm.provision "Install jq", type: "shell", inline: "zypper install -y jq"
    vm.provision "Install apparmor-parser", type: "shell", inline: "zypper install -y apparmor-parser"
  elsif box.include?("rocky8") || box.include?("rocky9")
    vm.provision "Install jq", type: "shell", inline: "dnf install -y jq"
    vm.provision "Disable firewall", type: "shell", inline: "systemctl stop firewalld"
  elsif box.include?("centos7")
    vm.provision "Install jq", type: "shell", inline: "yum install -y jq"
    vm.provision "Disable firewall", type: "shell", inline: "systemctl stop firewalld"
  elsif box.include?("alpine")
    vm.provision "Install tools", type: "shell", inline: "apk add jq coreutils"
  elsif box.include?("microos")
    vm.provision "Install jq", type: "shell", inline: "transactional-update pkg install -y jq"
    vm.provision 'reload', run: 'once'
  end 
end

def getInstallType(vm, release_version, branch)
  if release_version == "skip"
    install_type = "INSTALL_K3S_SKIP_DOWNLOAD=true"
  elsif !release_version.empty?
    return "INSTALL_K3S_VERSION=#{release_version}"
  else
    scripts_location = Dir.exists?("./scripts") ? "./scripts" : "../scripts" 
    # Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
    # MicroOS requires it not be in a /tmp/ or other root system folder
    vm.provision "Get latest commit", type: "shell", path: scripts_location +"/latest_commit.sh", args: [branch, "/tmp/k3s_commits"]
    return "INSTALL_K3S_COMMIT=$(head\ -n\ 1\ /tmp/k3s_commits)"
  end
end

def getHardenedArg(vm, hardened, scripts_location)
  if hardened.empty? 
    return ""
  end
  hardened_arg = <<~HARD
    protect-kernel-defaults: true
    secrets-encryption: true
    kube-controller-manager-arg:
      - 'terminated-pod-gc-threshold=10'
      - 'use-service-account-credentials=true'
    kubelet-arg:
      - 'streaming-connection-idle-timeout=5m'
      - 'make-iptables-util-chains=true'
      - 'event-qps=0'
    kube-apiserver-arg:
      - 'audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log'
      - 'audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml'
      - 'audit-log-maxage=30'
      - 'audit-log-maxbackup=10'
      - 'audit-log-maxsize=100'
      - 'service-account-lookup=true'
  HARD
  if hardened == "psp"
    vm.provision "Set kernel parameters", type: "shell", path: scripts_location + "/harden.sh"
    hardened_arg += "  - 'enable-admission-plugins=NodeRestriction,NamespaceLifecycle,ServiceAccount,PodSecurityPolicy'"
  elsif hardened == "psa"
    vm.provision "Set kernel parameters", type: "shell", path: scripts_location + "/harden.sh", args: [ "psa" ]
    hardened_arg += "  - 'admission-control-config-file=/var/lib/rancher/k3s/server/psa.yaml'"
  else 
    puts "Invalid E2E_HARDENED option"
    exit 1
  end
  return hardened_arg
end

def dockerInstall(vm)
  vm.provider "libvirt" do |v|
    v.memory = NODE_MEMORY + 1024
  end
  vm.provider "virtualbox" do |v|
    v.memory = NODE_MEMORY + 1024
  end
  box = vm.box.to_s
  if box.include?("ubuntu")
    vm.provision "shell", inline: "apt update; apt install -y docker.io"
  elsif box.include?("Leap")
    vm.provision "shell", inline: "zypper install -y docker apparmor-parser"
  elsif box.include?("microos")
    vm.provision "shell", inline: "transactional-update pkg install -y docker apparmor-parser"
    vm.provision 'docker-reload', type: 'reload', run: 'once'
    vm.provision "shell", inline: "systemctl enable --now docker"
  elsif box.include?("rocky8") || box.include?("rocky9")
    vm.provision "shell", inline: "dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo"
    vm.provision "shell", inline: "dnf install -y docker-ce"
  end
end
