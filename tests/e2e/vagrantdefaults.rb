def defaultOSConfigure(vm)
  box = vm.box.to_s
  if box.include?("generic/ubuntu")
    vm.provision "Set DNS", type: "shell", inline: "systemd-resolve --set-dns=8.8.8.8 --interface=eth0"
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