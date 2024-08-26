def defaultOSConfigure(vm)
  box = vm.box.to_s
  if box.include?("ubuntu")
    vm.provision "Set DNS", type: "shell", inline: "netplan set ethernets.eth0.nameservers.addresses=[8.8.8.8,1.1.1.1]; netplan apply", run: 'once'
  elsif box.include?("Leap") || box.include?("Tumbleweed")
    vm.provision "Install apparmor-parser", type: "shell", inline: "zypper install -y apparmor-parser"
  elsif box.include?("rocky") || box.include?("centos")
    vm.provision "Disable firewall", type: "shell", inline: "systemctl stop firewalld"
  elsif box.include?("alpine")
    vm.provision "Install tools", type: "shell", inline: "apk add coreutils"
  elsif box.include?("microos")
    # Add stuff here, but we always need to reload at the end
    vm.provision 'reload', run: 'once'
  end 
end

def getInstallType(vm, release_version, branch, release_channel='')
  if release_version == "skip"
    install_type = "INSTALL_K3S_SKIP_DOWNLOAD=true"
  elsif !release_version.empty?
    return "INSTALL_K3S_VERSION=#{release_version}"
  elsif !release_channel.empty? && release_channel != "commit"
    return "INSTALL_K3S_CHANNEL=#{release_channel}"
  else
    jqInstall(vm)
    scripts_location = Dir.exist?("./scripts") ? "./scripts" : "../scripts" 
    # Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
    # MicroOS requires it not be in a /tmp/ or other root system folder
    vm.provision "Get latest commit", type: "shell", path: scripts_location +"/latest_commit.sh", args: [branch, "/tmp/k3s_commits"]
    return "INSTALL_K3S_COMMIT=$(head\ -n\ 1\ /tmp/k3s_commits)"
  end
end

def addCoverageDir(vm, role, gocover)
  if gocover.empty?
    return
  end
  service = role.include?("agent") ? "k3s-agent" : "k3s" 
    script = <<~SHELL
      mkdir -p /tmp/k3scov
      echo -e 'GOCOVERDIR=/tmp/k3scov' >> /etc/default/#{service}
      systemctl daemon-reload
    SHELL
    vm.provision "go coverage", type: "shell", inline: script 
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
    kubelet-arg:
      - 'streaming-connection-idle-timeout=5m'
      - 'make-iptables-util-chains=true'
      - 'event-qps=0'
      - "tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305"
    kube-apiserver-arg:
      - 'audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log'
      - 'audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml'
      - 'audit-log-maxage=30'
      - 'audit-log-maxbackup=10'
      - 'audit-log-maxsize=100'
  HARD
 
  if hardened == "psa" || hardened == "true"
    vm.provision "Set kernel parameters", type: "shell", path: scripts_location + "/harden.sh", args: [ "psa" ]
    hardened_arg += "  - 'admission-control-config-file=/var/lib/rancher/k3s/server/psa.yaml'"
  elsif hardened == "psp"
      vm.provision "Set kernel parameters", type: "shell", path: scripts_location + "/harden.sh"
      hardened_arg += "  - 'enable-admission-plugins=NodeRestriction,NamespaceLifecycle,ServiceAccount,PodSecurityPolicy'"
  else 
    puts "Invalid E2E_HARDENED option"
    exit 1
  end
  if vm.box.to_s.include?("ubuntu")
    vm.provision "Install kube-bench", type: "shell", inline: <<-SHELL
    export KBV=0.8.0
    curl -L "https://github.com/aquasecurity/kube-bench/releases/download/v${KBV}/kube-bench_${KBV}_linux_amd64.deb" -o "kube-bench_${KBV}_linux_amd64.deb"
    dpkg -i "./kube-bench_${KBV}_linux_amd64.deb"
    SHELL
  end
  return hardened_arg
end

def jqInstall(vm)
  box = vm.box.to_s
  if box.include?("ubuntu")
    vm.provision "Install jq", type: "shell", inline: "apt install -y jq"
  elsif box.include?("Leap") || box.include?("Tumbleweed")
    vm.provision "Install jq", type: "shell", inline: "zypper install -y jq"
  elsif box.include?("rocky")
    vm.provision "Install jq", type: "shell", inline: "dnf install -y jq"
  elsif box.include?("centos")
    vm.provision "Install jq", type: "shell", inline: "yum install -y jq"
  elsif box.include?("alpine")
    vm.provision "Install jq", type: "shell", inline: "apk add coreutils"
  elsif box.include?("microos")
    vm.provision "Install jq", type: "shell", inline: "transactional-update pkg install -y jq"
    vm.provision 'reload', run: 'once'
  end 
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
  elsif box.include?("rocky")
    vm.provision "shell", inline: "dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo"
    vm.provision "shell", inline: "dnf install -y docker-ce"
  end
end
