def defaultOSConfigure(vm)
    if vm.box.include?("ubuntu2004")
      vm.provision "shell", inline: "systemd-resolve --set-dns=8.8.8.8 --interface=eth0"
      vm.provision "shell", inline: "apt install -y jq"
    end
    if vm.box.include?("Leap")
      vm.provision "shell", inline: "zypper install -y jq"
    end
    if vm.box.include?("microos")
      vm.provision "shell", inline: "transactional-update pkg install -y jq"
      vm.provision 'reload', run: 'once'
    end 
end