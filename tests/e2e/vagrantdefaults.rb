def defaultOSConfigure(vm)
  box = vm.box.to_s
  if box.include?("generic/ubuntu")
    vm.provision "Set DNS", type: "shell", inline: "netplan set ethernets.eth0.nameservers.addresses=[8.8.8.8,1.1.1.1]; netplan apply", run: 'once'
    vm.provision "Install jq", type: "shell", inline: "apt install -y jq"
  elsif box.include?("Leap") || box.include?("Tumbleweed")
    vm.provision "Install jq", type: "shell", inline: "zypper install -y jq"
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